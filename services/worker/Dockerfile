FROM python:3.11-slim

WORKDIR /app

COPY requirements.txt .

# Use host network during build so pip can reach PyPI in restricted Docker environments.
# The RUN --network=host flag is a BuildKit feature and is a no-op in standard builds.
RUN --mount=type=cache,target=/root/.pip-cache \
    pip install --no-cache-dir -r requirements.txt

COPY . .

CMD ["python", "-u", "main.py"]
