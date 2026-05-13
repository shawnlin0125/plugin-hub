# Dockerfile — plugin-hub with ticket-vendor
FROM python:3.12-slim

WORKDIR /app

# Install system deps
RUN pip install --no-cache-dir fastapi uvicorn pyyaml aiohttp apscheduler pytest pytest-asyncio

# Install platform SDK
COPY platform-plugin-sdk/ /app/sdk/
RUN pip install /app/sdk/

# Copy ticket-vendor source and install
COPY ../ticket-vendor/ /tmp/ticket-vendor/
RUN pip install /tmp/ticket-vendor/

# Copy platform core
COPY platform/ /app/platform/

ENV LOAD_PLUGINS="ticketmaster"
EXPOSE 8000

CMD ["python", "-m", "platform.main"]
