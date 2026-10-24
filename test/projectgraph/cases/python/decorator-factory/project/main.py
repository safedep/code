import click
import yaml


def verbose_option(f):
    return click.option("-v", "--verbose")(f)


@click.command()
@verbose_option
@click.option("-c", "--config")
def run(config):
    yaml.load(config)


if __name__ == "__main__":
    run()
