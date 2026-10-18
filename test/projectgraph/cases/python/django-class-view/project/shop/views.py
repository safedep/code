import subprocess

import yaml
from django.views import View


class ReportView(View):
    def get(self, request):
        return yaml.safe_load("a: 1")

    def post(self, request):
        subprocess.run(["make", "report"])


class UnusedView(View):
    def get(self, request):
        return yaml.full_load("b: 2")
