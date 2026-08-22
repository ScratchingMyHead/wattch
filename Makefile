PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
APP=wattch
PKG=github.com/ScratchingMyHead/wattch

build:
	go build -o $(APP) ./cmd/wattch
	go vet ./...

install: build
	install -Dm755 $(APP) $(BINDIR)/$(APP)
	install -Dm644 etc/99-rapl.rules $(HOME)/src/wattch/etc/99-rapl.rules
	@echo "Installed to $(BINDIR)/$(APP)"
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
