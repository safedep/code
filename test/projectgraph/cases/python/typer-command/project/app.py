import typer
import yaml

app = typer.Typer()


@app.command()
def load(path: str):
    yaml.load(path)


@app.callback()
def main():
    pass
