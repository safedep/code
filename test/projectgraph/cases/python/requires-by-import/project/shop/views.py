import subprocess

from django.views.generic import TemplateView


class ExportMixin(TemplateView):
    def get(self, request):
        subprocess.run(["export"])


class ReportView(ExportMixin):
    template_name = "report.html"
