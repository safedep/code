from django.urls import path

from .views import ReportView

urlpatterns = [path("report/", ReportView.as_view(template_name="r.html"))]
