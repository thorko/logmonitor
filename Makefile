all:
	go build ./cmd/logmonitor/logmonitor.go
	go build ./cmd/logmonctl/logmonctl.go

install: all
	install -m 0644 -D  etc/logmonitor/config.yaml /etc/logmonitor/config.yaml
	install -m 0755 -D logmonitor /usr/local/sbin/logmonitor
	install -m 0755 -D logmonctl /usr/local/bin/logmonctl
	install -m 0644 -D etc/systemd/system/logmonitor.service /etc/systemd/system/logmonitor.service

remove:
	rm -f /etc/systemd/system/logmonitor.service
	rm -f /usr/local/bin/logmonctl
	rm -f /usr/local/sbin/logmonitor