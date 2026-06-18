package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"labspectra/internal/httpapi"
	"labspectra/internal/service"
)

func main() {
	dataDir := flag.String("data", "lab_data", "путь к папке данных lab_data")
	addr := flag.String("addr", "127.0.0.1:8765", "адрес локального веб-сервера")
	noOpen := flag.Bool("no-open", false, "не открывать браузер автоматически")
	flag.Parse()

	svc, err := service.New(*dataDir)
	if err != nil {
		log.Fatalf("инициализация LabSpectra: %v", err)
	}
	defer svc.Close()

	srv, err := httpapi.New(svc)
	if err != nil {
		log.Fatalf("инициализация веб-сервера: %v", err)
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("не удалось занять адрес %s: %v", *addr, err)
	}
	url := "http://" + ln.Addr().String() + "/"

	fmt.Println("┌──────────────────────────────────────────────┐")
	fmt.Println("│  LabSpectra запущена                         │")
	fmt.Println("└──────────────────────────────────────────────┘")
	fmt.Printf("  Данные:    %s\n", svc.Root())
	fmt.Printf("  Интерфейс: %s\n", url)
	fmt.Println("  Остановить: Ctrl+C")

	if !*noOpen {
		go func() {
			time.Sleep(400 * time.Millisecond)
			openBrowser(url)
		}()
	}

	httpServer := &http.Server{Handler: srv.Handler()}
	if err := httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
		log.Fatalf("сервер остановлен с ошибкой: %v", err)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("не удалось открыть браузер автоматически, откройте вручную: %s", url)
	}
}
