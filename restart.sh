#!/bin/bash

dt=$(date '+%d-%m-%Y_%H-%M-%S');

kill $(pgrep -fi ./mychatbot) || true
nohup ./mychatbot > "chatbot_{$dt}.log" 2>&1 &
echo "chat bot restarted"
