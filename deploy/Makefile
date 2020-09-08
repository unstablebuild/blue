.PHONY: release-gps-server release-gps-producer release-logd-gps-server release-logd-gps-producer

cp-logd-files:
	@ cp ../deploy/logd/*.lua /etc/gps/logd && cd /etc/gps/logd && ./lit install

release-gps-server:
	@ systemctl stop gps-server
	@ cp bluectl gps-server /usr/local/bin
	@ cp ../deploy/systemd/gps-server.service /etc/systemd/system
	@ systemctl daemon-reload
	@ systemctl restart gps-server
	bluectl --help

release-logd-gps-server: cp-logd-files
	@ cp ../deploy/systemd/logd-gps-server.service /etc/systemd/system
	@ systemctl daemon-reload
	@ systemctl restart logd-gps-server


release-gps-producer:
	@ systemctl stop gps-producer
	@ cp bluectl gps-producer /usr/local/bin
	@ cp ../deploy/systemd/gps-producer.service /etc/systemd/system
	@ systemctl daemon-reload
	@ systemctl restart gps-producer
	bluectl --help

release-logd-gps-producer: cp-logd-files
	@ cp ../deploy/systemd/logd-gps-producer.service /etc/systemd/system
	@ systemctl daemon-reload
	@ systemctl restart logd-gps-producer
