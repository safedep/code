from setuptools import setup

setup(
    name="tool",
    entry_points={
        "console_scripts": [
            "tool = tool.main:cli",
            'tool-admin=tool.admin:run',
        ],
    },
)
