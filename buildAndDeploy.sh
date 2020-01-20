#!/usr/bin/env bash

echo "Building Alexa Bin Schedule Skill"

env GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" .

echo "Preparing zip package"
zip -j alexa-bin-schedule-skill.zip alexa-bin-schedule-skill

echo "Depoloying..."
aws lambda update-function-code --function-name AlexaBinScheduleSkill --zip-file fileb://alexa-bin-schedule-skill.zip

echo "Cleanup"
rm alexa-bin-schedule-skill
rm alexa-bin-schedule-skill.zip

echo "Done!"