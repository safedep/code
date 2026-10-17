from django.urls import path

from . import views

urlpatterns = [
    path("report/", views.ReportView.as_view(), name="report"),
]
