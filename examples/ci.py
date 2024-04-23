import os
import sys
import time

import redis


def motor_a(rds, power):
    rds.xadd("yakapi:prime:motor_a", {"power": power})


def motor_b(rds, power):
    rds.xadd("yakapi:prime:motor_b", {"power": power})


def apply_command(rds, cmd, args):
    cmd = cmd.strip().lower()
    args = args.split(" ")

    if cmd == "quit":
        return None
    elif cmd == "noop":
        return 0.0
    elif cmd == "lt":
        angle = args[0]
        motor_a(rds, -0.8)
        motor_b(rds, 0.8)

        return 1.32 * abs(float(angle) / 100)
    elif cmd == "rt":
        angle = args[0]
        motor_a(rds, 0.8)
        motor_b(rds, -0.8)

        return 1.32 * abs(float(angle)) / 100
    elif cmd == "fwd":
        duration = args[0]
        motor_a(rds, 0.8)
        motor_b(rds, 0.8)
        return float(duration) / 100
    elif cmd == "ffwd":
        duration = args[0]
        motor_a(rds, 1.0)
        motor_b(rds, 1.0)
        return float(duration) / 100
    elif cmd == "bck":
        duration = args[0]
        motor_a(rds, -0.8)
        motor_b(rds, -0.8)
        return float(duration) / 100
    else:
        raise Exception("Unknown command")

def send_result(rds, id, result, error=None):
    if error:
        response = {"error": error}
    else:
        response = {"result": result}

    rds.xadd("yakapi:ci:result", response, id=id)
    print(f"sent response: {response}")

def main():
    rds = redis.from_url(os.environ.get(
        "REDIS_URL", "redis://localhost:6379/0"))
    if not rds.ping():
        print("Redis is not available", file=sys.stderr)
        sys.exit(1)

    streams = {
        b"yakapi:ci": "$"
    }

    while True:
        try:
            for stream, evts in rds.xread(streams, block=100):
                for evt in evts:
                    # Continue from the last event
                    streams[stream] = evt[0]

                    fields = ""
                    command = {}
                    for k, v in evt[1].items():
                        command[k.decode()] = v.decode()
                        fields += "{}={} ".format(k.decode(), v.decode())
                    print("{} {} {}".format(
                        stream.decode(), evt[0].decode(), fields))

                    print(f"processing {command["cmd"]} {command["args"]}... ", end="", flush=True)
                    try:
                        next_delay = apply_command(rds, command["cmd"], command.get("args", ""))
                    except Exception as e:
                        send_result(rds, evt[0], None, error=str(e))
                        continue

                    print(f"sleeping")
                    time.sleep(next_delay)
                    motor_a(rds, 0)
                    motor_b(rds, 0)
                    print("done")

                    send_result(rds, evt[0], "ok")

        except KeyboardInterrupt:
            print("Bye!")
            break

if __name__ == "__main__":
    main()