"""Entry point for the doc-intel worker service."""

import logging
import sys

from config import Config
from worker import Worker


def main() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)-8s [%(name)s] %(message)s",
        stream=sys.stdout,
        force=True,
    )

    cfg = Config()
    w = Worker(cfg)
    w.run()


if __name__ == "__main__":
    main()
