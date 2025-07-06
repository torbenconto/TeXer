package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/torbenconto/TeXer/internal/watcher"
	"gopkg.in/yaml.v3"
)

var (
	path     string
	interval time.Duration
	clean    bool
)

func main() {
	flag.StringVar(&path, "path", "./", "root path to watch")
	flag.DurationVar(&interval, "interval", 50*time.Millisecond, "polling interval")
	flag.BoolVar(&clean, "clean", false, "clean artifacts after build")
	flag.Parse()

	candidates := []string{"pdflatex", "xelatex", "lualatex", "latexmk", "tectonic"}
	var found []string
	for _, c := range candidates {
		if p, err := exec.LookPath(c); err == nil {
			found = append(found, filepath.Base(p))
		}
	}
	if len(found) == 0 {
		fmt.Println("No TeX compilers found in PATH.")
		os.Exit(1)
	}

	cfgPath := ".texer.yml"
	cfg := Config{}
	if data, err := os.ReadFile(cfgPath); err == nil {
		_ = yaml.Unmarshal(data, &cfg)
	} else if home, err := os.UserHomeDir(); err == nil {
		globalPath := filepath.Join(home, ".texer.yml")
		if data, err := os.ReadFile(globalPath); err == nil {
			_ = yaml.Unmarshal(data, &cfg)
			cfgPath = globalPath
		}
	}

	if cfg.Compiler == "" {
		_, compiler, err := (&promptui.Select{
			Label: "Select TeX compiler",
			Items: found,
		}).Run()
		if err != nil {
			fmt.Println("Compiler selection cancelled.")
			os.Exit(1)
		}
		cfg.Compiler = compiler
		if home, err := os.UserHomeDir(); err == nil {
			cfgPath = filepath.Join(home, ".texer.yml")
		}
		if data, err := yaml.Marshal(&cfg); err == nil {
			_ = os.WriteFile(cfgPath, data, 0644)
			fmt.Println("Selected compiler saved to", cfgPath)
		}
	} else {
		fmt.Println("Using compiler from config:", cfg.Compiler)
	}

	w := watcher.NewWatcher(interval, func(s string) bool {
		return !strings.HasSuffix(s, ".tex")
	})

	defer w.Close()
	if err := w.Add(path); err != nil {
		fmt.Println("Failed to watch path:", err)
		os.Exit(1)
	}
	w.Start()

	for {
		select {
		case e := <-w.Events():
			if e.Type&(watcher.EventCreate|watcher.EventModify) != 0 {
				if !clean {
					exec.Command(cfg.Compiler, e.Path).Run()
					continue
				}

				exec.Command(cfg.Compiler, e.Path).Run()

				exts := []string{".aux", ".log", ".out", ".toc", ".fls", ".fdb_latexmk"}
				filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
					if err == nil && !info.IsDir() {
						for _, ext := range exts {
							if strings.HasSuffix(path, ext) {
								os.Remove(path)
							}
						}
					}
					return nil
				})
			} else if e.Type&watcher.EventDelete != 0 {
				exts := []string{".pdf", ".aux", ".log", ".toc", ".out"}
				base := strings.TrimSuffix(e.Path, filepath.Ext(e.Path))
				for _, ext := range exts {
					os.Remove(base + ext)
				}
			}
		}
	}
}
