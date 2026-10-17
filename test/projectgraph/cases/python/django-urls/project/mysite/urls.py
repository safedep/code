from django.urls import path

from . import views
from .views import health

urlpatterns = [
    path("", views.home, name="home"),
    path("health/", health),
]
