## Prerrequisitos
- Instalar Docker y Docker Compose en tu máquina macOS.
- Si usas Windows, copia el archivo **windows.env.dist** a **windows.env** y modifica las variables de entorno según sea necesario.
- Instalar y configurar XQuartz (MacOS):
  - Descarga e instala XQuartz desde https://www.xquartz.org/.
  Abre XQuartz y ve a XQuartz > Preferences > Security, y marca la casilla Allow connections from network clients.
- Instalar y configurar VcXsrv (Windows):
  - Descarga e instala VcXsrv desde https://sourceforge.net/projects/vcxsrv/.
    Ejecuta XLaunch (parte de VcXsrv) y selecciona las siguientes opciones:
    Selecciona "Multiple windows" y display number = 0.
    Selecciona "Start no client".
    Desmarca "Native opengl" y marca "Disable access control".
    Guarda la configuración si deseas, y luego haz clic en "Finish".

- Permite acceso desde localhost con el siguiente comando (MacOs):


      xhost + 127.0.0.1



## Pasos detallados para MACOS
- Inicia XQuartz:

  - Abre XQuartz desde tu carpeta de Aplicaciones.
  Asegúrate de que esté ejecutándose.
  Permite acceso desde localhost. Abre una terminal en macOS y ejecuta:

        xhost + 127.0.0.1


- Configura la variable de entorno DISPLAY:

  - Asegúrate de que la variable de entorno DISPLAY esté configurada correctamente. Puedes hacer esto en la misma terminal donde ejecutarás Docker Compose:
  
        export DISPLAY=host.docker.internal:0

  - Verifica que host.docker.internal esté accesible:

- Docker para Mac proporciona esta dirección para acceder a la máquina host desde el contenedor. Asegúrate de que esta dirección esté disponible y correcta.
Construye y ejecuta el contenedor con Docker Compose:

  - Navega al directorio donde se encuentran tu Dockerfile y docker-compose.yml.
  Construye y ejecuta los servicios con Docker Compose:

        make os=macos up   # desde la carpeta docker/

## Pasos detallados para Windows

- Copia el archivo **windows.env.dist** a **windows.env**.
- Ejecuta XLaunch (parte de VcXsrv) y selecciona las siguientes opciones:
  Selecciona "Multiple windows" y display number = 0.
  Selecciona "Start no client".
  Marca "Native opengl" y marca "Disable access control".
  Guarda la configuración si deseas, y luego haz clic en "Finish".
- Lanza la aplicación con Makefile, lo cual actualizará de forma automática las variables windows.env para concuerden con tu ipv4 y así poder conectar la pantalla:

      make os=windows update-display  
      make os=windows up

## Hacer build de la aplicación
- Si hemos cambiado de arquitectura, el primer paso será borrar node modules y package.lock.json:

      cd frontend
      rm -rf node_modules package-lock.json

- Luego, ejecuta el siguiente comando:

      npm install
      npm run build

- Para construir la aplicación, ejecuta el siguiente comando:

      wails build