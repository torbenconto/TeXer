package watcher

import (
	"os"
	"path/filepath"
	"time"
)

type Node struct {
	Path     string
	IsDir    bool
	Size     int64
	Modified time.Time
	Children map[string]*Node
}

func buildTree(root string, ignore func(string) bool) (*Node, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}

	n := &Node{
		Path:     root,
		IsDir:    info.IsDir(),
		Size:     info.Size(),
		Modified: info.ModTime(),
	}

	if !n.IsDir {
		return n, nil
	}

	n.Children = make(map[string]*Node)
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		p := filepath.Join(root, entry.Name())
		if ignore != nil && ignore(p) {
			continue
		}
		child, err := buildTree(p, ignore)
		if err != nil {
			continue
		}
		n.Children[entry.Name()] = child
	}

	return n, nil
}

func eq(a, b *Node) bool {
	return a.IsDir == b.IsDir &&
		a.Size == b.Size &&
		a.Modified.Equal(b.Modified)
}

func diff(t1, t2 *Node, events chan Event) {
	for name, child := range t1.Children {
		if _, ok := t2.Children[name]; !ok {
			recursiveEmit(child, EventDelete, events, false)
		}
	}

	for name, new := range t2.Children {
		old, existed := t1.Children[name]

		if !existed {
			recursiveEmit(new, EventCreate, events, false)
		} else if !eq(new, old) {
			events <- Event{
				Path:      new.Path,
				Type:      EventModify,
				IsDir:     new.IsDir,
				Timestamp: new.Modified,
			}
		}

		if new.IsDir && existed && old.IsDir {
			diff(old, new, events)
		}
	}
}

func recursiveEmit(node *Node, event EventType, events chan Event, skipRoot bool) {
	if !skipRoot {
		events <- Event{
			Path:      node.Path,
			Type:      event,
			IsDir:     node.IsDir,
			Timestamp: node.Modified,
		}
	}
	if node.IsDir {
		for _, child := range node.Children {
			recursiveEmit(child, event, events, false)
		}
	}
}
