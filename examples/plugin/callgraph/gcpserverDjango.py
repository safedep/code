import os
import sys
import django
from django.conf import settings
from django.http import JsonResponse
from django.urls import path
from django.views.decorators.csrf import csrf_exempt
from django.core.management import execute_from_command_line
import json

from google import cloud
import google.cloud.storage as gcpstorage
from google.cloud import bigquery, pubsub_v1, secretmanager, translate_v2 as translatergcp
from google.oauth2 import service_account

# Django configuration
settings.configure(
    DEBUG=True,
    ROOT_URLCONF=__name__,
    SECRET_KEY="dummy",
    ALLOWED_HOSTS=["*"],
    MIDDLEWARE=[
        "django.middleware.common.CommonMiddleware",
    ],
)

django.setup()

# GCP Base Service
class BaseGCPService:
    def __init__(self, config: dict):
        credentials_path = os.environ.get("GOOGLE_APPLICATION_CREDENTIALS")
        if not credentials_path:
            raise EnvironmentError("GOOGLE_APPLICATION_CREDENTIALS env var not set")
        self.credentials = service_account.Credentials.from_service_account_file(credentials_path)
        self.config = config

class GCPStorageServices(BaseGCPService):
    def __init__(self, config: dict):
        super().__init__(config)
        self.storage_client = gcpstorage.Client(credentials=self.credentials)
        self.bq_client = bigquery.Client(credentials=self.credentials)
        self.firestore_client = cloud.firestore.Client(credentials=self.credentials)
        self.secret_client = secretmanager.SecretManagerServiceClient(credentials=self.credentials)

    def get_file_url(self, bucket_name, blob_name):
        bucket = self.storage_client.bucket(bucket_name)
        blob = bucket.blob(blob_name)
        return blob.public_url

    def run_bq_query(self, query):
        query_job = self.bq_client.query(query)
        return [dict(row.items()) for row in query_job.result()]

    def add_firestore_document(self, collection, doc_id, data):
        doc_ref = self.firestore_client.collection(collection).document(doc_id)
        doc_ref.set(data)
        return f"Document {doc_id} added to {collection}"

    def get_secret(self, secret_id, version="latest"):
        name = f"projects/{self.config['project_id']}/secrets/{secret_id}/versions/{version}"
        response = self.secret_client.access_secret_version(request={"name": name})
        return response.payload.data.decode("UTF-8")

class GCPAiServices(BaseGCPService):
    def __init__(self, config: dict):
        super().__init__(config)
        self.translate_client = translatergcp.Client(credentials=self.credentials)

    def translate_text(self, text, target="en"):
        result = self.translate_client.translate(text, target_language=target)
        return result['translatedText']

class GCPMessagingServices(BaseGCPService):
    def __init__(self, config: dict):
        super().__init__(config)
        self.pubsub_publisher = pubsub_v1.PublisherClient(credentials=self.credentials)

    def publish_message(self, topic_name, message):
        topic_path = self.pubsub_publisher.topic_path(self.config['project_id'], topic_name)
        future = self.pubsub_publisher.publish(topic_path, message.encode("utf-8"))
        return future.result()

# Global instances
config = {
    "project_id": os.environ.get("GCP_PROJECT_ID", "your-gcp-project-id")
}
storage_services = GCPStorageServices(config)
ai_services = GCPAiServices(config)
messaging_services = GCPMessagingServices(config)

# Views
def get_file_url(request):
    bucket = request.GET.get("bucket")
    blob = request.GET.get("blob")
    url = storage_services.get_file_url(bucket, blob)
    return JsonResponse({"url": url})

@csrf_exempt
def bigquery_query(request):
    data = json.loads(request.body)
    query = data.get("query")
    result = storage_services.run_bq_query(query)
    return JsonResponse(result, safe=False)

@csrf_exempt
def pubsub_publish(request):
    data = json.loads(request.body)
    topic = data.get("topic")
    message = data.get("message")
    msg_id = messaging_services.publish_message(topic, message)
    return JsonResponse({"message_id": msg_id})

@csrf_exempt
def firestore_add(request):
    data = json.loads(request.body)
    collection = data.get("collection")
    doc_id = data.get("doc_id")
    doc_data = data.get("data")
    status = storage_services.add_firestore_document(collection, doc_id, doc_data)
    return JsonResponse({"status": status})

def secret_get(request):
    secret_id = request.GET.get("secret_id")
    version = request.GET.get("version", "latest")
    secret = storage_services.get_secret(secret_id, version)
    return JsonResponse({"secret": secret})

@csrf_exempt
def translate_text(request):
    data = json.loads(request.body)
    text = data.get("text")
    target = data.get("target", "en")
    translated = ai_services.translate_text(text, target)
    return JsonResponse({"translated": translated})

# URL Patterns
urlpatterns = [
    path("storage/url", get_file_url),
    path("bigquery/query", bigquery_query),
    path("pubsub/publish", pubsub_publish),
    path("firestore/add", firestore_add),
    path("secret/get", secret_get),
    path("translate", translate_text),
]

# Run server: `python gcp_service_app.py runserver 8000`
if __name__ == "__main__":
    execute_from_command_line(sys.argv)
