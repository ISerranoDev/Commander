## Prerrequisitos
- Instalar Docker y Docker Compose en tu máquina macOS.
- Instalar y configurar XQuartz (MacOS):
  - Descarga e instala XQuartz desde https://www.xquartz.org/.
  Abre XQuartz y ve a XQuartz > Preferences > Security, y marca la casilla Allow connections from network clients.
- Instalar y configurar VcXsrv (Windows):
  - Descarga e instala VcXsrv desde https://sourceforge.net/projects/vcxsrv/.
    Ejecuta XLaunch (parte de VcXsrv) y selecciona las siguientes opciones:
    Selecciona "Multiple windows".
    Selecciona "Start no client".
    Desmarca "Native opengl" y marca "Disable access control".
    Guarda la configuración si deseas, y luego haz clic en "Finish".

- Permite acceso desde localhost con el siguiente comando (MacOs):


      xhost + 127.0.0.1

- Crea un Dockerfile para tu aplicación GUI. Usaremos xeyes como ejemplo:


      FROM ubuntu:20.04
      
      RUN apt-get update && \
      apt-get install -y x11-apps
      
      CMD ["xeyes"]
- Crea un archivo docker-compose.yml en el mismo directorio que tu Dockerfile:


    services:
        app:
            build: .
            environment:
                - DISPLAY=${DISPLAY}
            volumes:
                - /tmp/.X11-unix:/tmp/.X11-unix


## Pasos detallados
- Inicia XQuartz o VcXsrv:

  - Abre XQuartz o VcXsrv desde tu carpeta de Aplicaciones.
  Asegúrate de que esté ejecutándose.
  Permite acceso desde localhost. Abre una terminal en macOS y ejecuta:

        xhost + 127.0.0.1


- Configura la variable de entorno DISPLAY:

  - Asegúrate de que la variable de entorno DISPLAY esté configurada correctamente. Puedes hacer esto en la misma terminal donde ejecutarás Docker Compose:
    - Para macOS:
  
          export DISPLAY=host.docker.internal:0

    - Para Windows:

          $Env:DISPLAY="host.docker.internal:0.0"

  - Verifica que host.docker.internal esté accesible:

- Docker para Mac proporciona esta dirección para acceder a la máquina host desde el contenedor. Asegúrate de que esta dirección esté disponible y correcta.
Construye y ejecuta el contenedor con Docker Compose:

  - Navega al directorio donde se encuentran tu Dockerfile y docker-compose.yml.
  Construye y ejecuta los servicios con Docker Compose:

        docker-compose up
