import os
import sys
import time

import redis

def send_telemetry(rds, key, value):
    response = {key: str(value)}

    rds.xadd("yakapi:telemetry", response, maxlen=1000)
    print(f"sent response: {response}")

def main():
    redis_url = os.environ.get("YAKAPI_REDIS_URL", "localhost:6379")
    rds = redis.from_url(os.environ.get(
        "REDIS_URL", f"redis://{redis_url}/0"))
    if not rds.ping():
        print("Redis is not available", file=sys.stderr)
        sys.exit(1)

    startTime = time.time()
    while True:
        try:
            send_telemetry(rds, "uptime", time.time() - startTime)
            time.sleep(1)
        except KeyboardInterrupt:
            print("Bye!")
            break

if __name__ == "__main__":
    main()