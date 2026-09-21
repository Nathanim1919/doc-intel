import os
import time
import sys

def main():
    print("Worker service initialized. Waiting for jobs...", flush=True)
    while True:
        time.sleep(10)

if __name__ == "__main__":
    main()
