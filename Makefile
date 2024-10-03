# Levanta la arquitectura

file_selected := -f docker-compose.$(os).yml

update-display:
	# Define el comando para obtener la dirección IP
	HOST_IP := $(shell powershell -Command "(ipconfig | Select-String 'IPv4' | Select-Object -First 1) -replace '.*: ', ''")
	# Combina la IP con :0
	DISPLAY := $(HOST_IP):0
	@powershell -Command "(Get-Content windows.env) -replace 'DISPLAY=.*', 'DISPLAY=$(DISPLAY)' | Set-Content windows.env"

up:
	@docker-compose $(file_selected) up -d

ps:
	@docker-compose $(file_selected) ps

down:
	@docker-compose $(file_selected) down

build:
	@docker-compose $(file_selected) build $(c)

restart:
	@docker-compose $(file_selected) restart $(c)

logs:
	@docker-compose $(file_selected) logs -f $(c)

connect:
	@docker-compose $(file_selected) exec $(c) bash

