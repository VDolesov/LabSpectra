# Деплой на Railway

Приложение читает порт из `PORT` и папку данных из `DATA_DIR`, поэтому на
Railway достаточно собрать `Dockerfile`.

## Шаги

1. Railway → **New Project → Deploy from GitHub repo** → выбрать `VDolesov/LabSpectra`.
   Railway увидит `Dockerfile` и соберёт образ сам.
2. **Volume** (обязательно, иначе данные пропадут при передеплое):
   Railway → сервис → **Volumes → New Volume**, путь монтирования **`/data`**.
   Приложение хранит всё в `DATA_DIR=/data` (задано в `Dockerfile`).
3. **Переменные окружения** (Variables):
   - `ADMIN_PASSWORD` — пароль администратора. **Обязательно поменять** с дефолтного `123`.
   - `PORT` Railway задаёт сам — трогать не нужно.
4. Deploy. Публичный URL появится в разделе **Settings → Networking → Generate Domain**.

## Важно про безопасность

По текущему решению **данные анализов открыты всем по ссылке** — кто угодно может
смотреть, создавать и редактировать. Паролем закрыт только раздел «Управление»
(подтверждение/откат удаления). Если это не нужно — не публикуйте домен или
добавьте защиту на весь вход отдельно.

## Локальный запуск в Docker

```sh
docker build -t labspectra .
docker run -p 8080:8080 -e PORT=8080 -e ADMIN_PASSWORD=секрет -v labspectra-data:/data labspectra
```
Откройте http://localhost:8080
