package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

const (
	udpPort = ":12345"
	vpnPort = ":8443"
)

type ServerInfo struct {
	Name     string
	LastSeen time.Time
}

var (
	discoveredServers = make(map[string]ServerInfo)
	mutex             sync.Mutex
	isServerRunning   = false
)

func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("Go VPN Multiplayer")
	myWindow.Resize(fyne.NewSize(500, 500))

	hostname, _ := os.Hostname()
	myIP := getLocalIP()

	infoLabel := widget.NewLabel(fmt.Sprintf("Ваш Компьютер:\nИмя ПК: %s\nЛокальный IP: %s", hostname, myIP))

	logBox := widget.NewMultiLineEntry()
	logBox.SetMinRowsVisible(6)
	logBox.Text = "Лог работы / Статус:\nСистема готова.\n"
	logBox.Refresh()

	appendLog := func(msg string) {
		logBox.Text += msg + "\n"
		logBox.Refresh()
	}

	serverList := widget.NewList(
		func() int {
			mutex.Lock()
			defer mutex.Unlock()
			return len(discoveredServers)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			mutex.Lock()
			defer mutex.Unlock()
			keys := make([]string, 0, len(discoveredServers))
			for k := range discoveredServers {
				keys = append(keys, k)
			}
			ip := keys[i]
			info := discoveredServers[ip]
			o.(*widget.Label).SetText(fmt.Sprintf("%s - %s", info.Name, ip))
		},
	)

	go func() {
		addr, _ := net.ResolveUDPAddr("udp", udpPort)
		conn, err := net.ListenUDP("udp", addr)
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 1024)
		for {
			n, remoteAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				continue
			}
			msg := string(buf[:n])
			if strings.HasPrefix(msg, "VPN_SERVER:") {
				srvName := strings.TrimPrefix(msg, "VPN_SERVER:")
				mutex.Lock()
				discoveredServers[remoteAddr.IP.String()] = ServerInfo{
					Name:     srvName,
					LastSeen: time.Now(),
				}
				mutex.Unlock()
			}
		}
	}()

	go func() {
		for {
			time.Sleep(2 * time.Second)
			mutex.Lock()
			now := time.Now()
			for ip, info := range discoveredServers {
				if now.Sub(info.LastSeen) > 6*time.Second {
					delete(discoveredServers, ip)
				}
			}
			mutex.Unlock()
			serverList.Refresh()
		}
	}()

	var createBtn *widget.Button
	createBtn = widget.NewButton("[ Create Server ]", func() {
		if isServerRunning {
			return
		}
		isServerRunning = true
		createBtn.SetText("[ Server Running... ]")
		createBtn.Disable()
		appendLog(fmt.Sprintf("[Сервер]: Запуск VPN на %s%s...", myIP, vpnPort))

		go func() {
			addr, _ := net.ResolveUDPAddr("udp", "255.255.255.255"+udpPort)
			conn, _ := net.DialUDP("udp", nil, addr)
			defer conn.Close()
			for isServerRunning {
				conn.Write([]byte("VPN_SERVER:" + hostname))
				time.Sleep(2 * time.Second)
			}
		}()

		go func() {
			l, _ := net.Listen("tcp", vpnPort)
			defer l.Close()
			for {
				conn, err := l.Accept()
				if err != nil {
					continue
				}
				appendLog(fmt.Sprintf("[Сервер]: Игрок %s успешно подключился!", conn.RemoteAddr().String()))
				conn.Close()
			}
		}()
	})

	mainLayout := container.NewVBox(
		infoLabel,
		logBox,
		widget.NewLabel("Servers:"),
		container.NewGridWithRows(1, serverList),
		createBtn,
	)

	myWindow.SetContent(mainLayout)
	myWindow.ShowAndRun()
}
