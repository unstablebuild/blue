.PHONY: release-gps-server release-gps-producer release-logd-gps-server

release-gps-server:
	@ systemctl stop gps-server
	@ cp bluectl gps-server /usr/local/bin
	@ cp ../deploy/systemd/gps-server.service /etc/systemd/system
	@ systemctl daemon-reload
	@ systemctl start gps-server
	bluectl --help

release-logd-gps-server:
	@ cp ../deploy/logd/*.lua /etc/gps/logd && cd /etc/gps/logd && ./lit install
	@ systemctl restart logd-gps-server

release-gps-producer:
	@ systemctl stop gps-producer
	@ cp bluectl gps-producer /usr/local/bin
	@ cp ../deploy/systemd/gps-producer.service /etc/systemd/system
	@ systemctl daemon-reload
	@ systemctl start gps-producer
	bluectl --help
