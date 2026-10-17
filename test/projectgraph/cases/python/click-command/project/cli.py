import click
import yaml


@click.group()
def cli():
    pass


@cli.command()
def load():
    yaml.load("a: 1")


@click.command()
def standalone():
    pass
