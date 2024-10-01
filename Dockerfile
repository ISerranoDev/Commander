# Usar la imagen base de golang 1.21-alpine
FROM golang:1.23.1
#FROM golang:1.23.1-alpine

# Establecer el directorio de trabajo dentro del contenedor
WORKDIR /app

# Instalar dependencias necesarias (bash, git, etc.)
RUN apt-get update && apt-get install -y bash git curl x11-apps

# Instalar dependencias necesarias
RUN apt-get update && apt-get install -y \
    bash \
    git \
    curl \
    x11-apps \
    build-essential \
    libgtk-3-dev \
    libglib2.0-dev \
    libwebkit2gtk-4.0-dev \
    xorg-dev \
    pkg-config \
    gcc \
    g++ \
    nodejs \
    npm \
    dbus-x11 \
    libxxf86vm-dev \
    && apt-get clean && ldconfig

RUN go install github.com/wailsapp/wails/v2/cmd/wails@latest


# Copiar el código de la aplicación (puedes modificar según tu estructura)
COPY . /app

# Establecer las variables de entorno necesarias
ENV ENV=development

# Exponer el puerto en el que correrá la aplicación
EXPOSE 4000

# Entrar en bash al iniciar el contenedor
CMD ["sh"]
