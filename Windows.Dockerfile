# Usar la imagen base de golang 1.21-alpine
FROM golang:1.23.1-bullseye
#FROM golang:1.23.1-alpine

# Establecer el directorio de trabajo dentro del contenedor
WORKDIR /app

RUN apt-get update -o Acquire::ForceIPv4=true && apt-get install -y \
    bash \
    git \
    curl \
    x11-apps \
    build-essential \
    xorg-dev \
    pkg-config \
    gcc \
    g++ \
    dbus-x11 \
    libxxf86vm-dev \
    mingw-w64 \
    && curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y nodejs \
    && apt-get clean && ldconfig


RUN apt-get update && apt-get install -y \
    libgtk-3-dev \
    libglib2.0-dev \
    libwebkit2gtk-4.0-dev \
    gir1.2-gtk-3.0 \
    gir1.2-webkit2-4.0 \
    && apt-get clean && ldconfig



RUN go install github.com/wailsapp/wails/v2/cmd/wails@latest


# Copiar el código de la aplicación (puedes modificar según tu estructura)
COPY . /app

# Establecer las variables de entorno necesarias
ENV ENV=development

# Entrar en bash al iniciar el contenedor
CMD ["sh"]
