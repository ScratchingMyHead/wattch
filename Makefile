PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
APP=wattch
PKG=github.com/ScratchingMyHead/wattch

build:
	go build -o $(APP) ./cmd/wattch
	go vet ./...

install: build
	install -Dm755 $(APP) $(BINDIR)/$(APP)
	install -Dm644 etc/wattch.desktop $(PREFIX)/share/applications/wattch.desktop
	for s in 16 22 24 32 48 64 128 256; do \
		install -Dm644 etc/icons/hicolor/$${s}x$${s}/apps/wattch.png $(PREFIX)/share/icons/hicolor/$${s}x$${s}/apps/wattch.png; \
	done
	install -Dm644 etc/icons/hicolor/scalable/apps/wattch.svg $(PREFIX)/share/icons/hicolor/scalable/apps/wattch.svg
	-gtk-update-icon-cache -f -t $(PREFIX)/share/icons/hicolor 2>/dev/null || true
	-update-desktop-database $(PREFIX)/share/applications 2>/dev/null || true
	@echo "Installed to $(BINDIR)/$(APP) and desktop/icon to $(PREFIX)/share"
	@echo "To enable RAPL without sudo, run:"
	@echo "  sudo cp etc/99-rapl.rules /etc/udev/rules.d/ && sudo udevadm control --reload-rules && sudo udevadm trigger --subsystem-match=powercap"

install-system: build
	sudo cp etc/99-rapl.rules /etc/udev/rules.d/99-rapl.rules
	sudo udevadm control --reload-rules
	sudo udevadm trigger --subsystem-match=powercap || true
	sudo chmod 444 /sys/class/powercap/intel-rapl*/energy_uj 2>/dev/null || true

clean:
	rm -f $(APP)
	go clean

vet:
	go vet ./...

fmt:
	go fmt ./...
