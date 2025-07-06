package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gosuri/uilive"
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

	writer := uilive.New()
	writer.Start()
	defer writer.Stop()

	startTime := time.Now()
	var lastFile, lastStatus string

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			printStatus(writer, lastFile, lastStatus, time.Since(startTime))
		case e := <-w.Events():
			if e.Type&(watcher.EventCreate|watcher.EventModify) != 0 {
				lastFile = filepath.Base(e.Path)
				lastStatus = fmt.Sprintf("\033[34m⚙ Compiling %s...\033[0m", lastFile)

				cmd := exec.Command(cfg.Compiler, e.Path)
				if err := cmd.Run(); err != nil {
					lastStatus = fmt.Sprintf("\033[31m✖ Compile error: %v\033[0m", err)
				} else {
					lastStatus = fmt.Sprintf("\033[32m✔ Compiled: %s\033[0m", lastFile)
				}

				if clean {
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
				}
			} else if e.Type&watcher.EventDelete != 0 {
				lastFile = filepath.Base(e.Path)
				lastStatus = fmt.Sprintf("\033[33m⚠ Deleted: %s\033[0m", lastFile)
				exts := []string{".pdf", ".aux", ".log", ".toc", ".out"}
				base := strings.TrimSuffix(e.Path, filepath.Ext(e.Path))
				for _, ext := range exts {
					os.Remove(base + ext)
				}
			}
		}
	}
}

func printStatus(writer *uilive.Writer, lastFile, lastStatus string, elapsed time.Duration) {
	var emoji string
	if clean {
		emoji = "🫧"
	}

	header := fmt.Sprintf("\n\n\033[1;35m ⟪ texer v1 %s ⟫\033[0m\n", emoji)

	fmt.Fprintf(writer,
		"%s\n"+
			" \033[38;5;81m📂 Watching:\033[0m  %s\n"+
			" \033[38;5;81m⏱ Uptime:\033[0m    %s\n\n"+
			" \033[38;5;222m📌 Last file:\033[0m %s\n"+
			" \033[38;5;114m✔ Status:\033[0m     %s\n",

		header,
		path,
		formatDuration(elapsed),
		lastFile,
		lastStatus,
	)
}

func formatDuration(d time.Duration) string {
	return fmt.Sprintf("%02d:%02d:%02d",
		int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
}
