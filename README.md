# linewatch

Doce medidores, catorce días de lecturas horarias y un análisis que clasifica anomalías. Go sirve la API. React muestra el recorrido.

## Requisitos

Go 1.27 y Node con npm.

## Variables

Crea un `.env` en la raíz. El proceso lo lee al arrancar y no pisa variables que ya existan.

```
JWT_SECRET=una-cadena-larga-y-aleatoria
OPENROUTER_API_KEY=
PORT=8082
```

`JWT_SECRET` es obligatorio. Sin él el servidor no arranca. Firma el JWT con HS256.

`OPENROUTER_API_KEY` es opcional. Con una clave válida, el modelo redacta la explicación y la acción a partir de las cifras que ya calculó Go. Si la variable está vacía, la clave no sirve o la llamada falla, queda la plantilla con esas mismas cifras. La clase, la severidad y la confianza no cambian.

`PORT` es el puerto de la API. Si se omite, usa `8082`. La interfaz en desarrollo reenvía las peticiones a ese puerto.

## Arranque

En una terminal, la API:

```bash
go test ./...
go run ./cmd/linewatch
```

Al subir carga `data/readings.csv` y `data/events.csv` en `linewatch.db`. `GET http://localhost:8082/health` responde `{"status":"ok"}`. El resto de rutas exige `Authorization: Bearer`.

En otra terminal, la interfaz:

```bash
npm install --prefix web
npm run dev --prefix web
```

Abre `http://localhost:5173`.

## Demo

Usuario `demo`, contraseña `linewatch-demo`. El selector de idioma guarda español o inglés en el navegador. Salir borra el token.

## Qué mirar en la grabación

Login → Tablero → M-109 → Run AI Analysis → Anomalía → Explicación → Acción.

Después del análisis: 4 anomalías, 2 de alta prioridad. M-109 es anomalía real y va primero. M-112 es calidad de datos, con lecturas eléctricas que no cierran. M-104 es explicable por la línea de producción nueva. M-106 es un falso positivo por la parada programada y no se escala.
