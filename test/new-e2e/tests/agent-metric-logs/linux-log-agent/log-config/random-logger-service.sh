#!/bin/bash
cat > random-logger.service << EOF
[Unit]
Description=Random logger
[Service]
ExecStart=/usr/bin/random-logger.py
[Install]
WantedBy=multi-user.target
EOF
sudo mv random-logger.service /etc/systemd/system/random-logger.service
sudo chmod 644 /etc/systemd/system/random-logger.service
