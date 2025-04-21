import os
from flask import Flask, request, jsonify
from google import cloud
import google.cloud.storage as gcpstorage
from google.cloud import bigquery, pubsub_v1, secretmanager, translate_v2 as translatergcp
from google.oauth2 import service_account

class GCPServices:
    def __init__(self, config: dict):
        # Resolve credentials from environment variable
        credentials_path = os.environ.get("GOOGLE_APPLICATION_CREDENTIALS")
        if not credentials_path:
            raise EnvironmentError("GOOGLE_APPLICATION_CREDENTIALS env var not set")

        credentials = service_account.Credentials.from_service_account_file(credentials_path)

        self.config = config
        self.storage_client = gcpstorage.Client(credentials=credentials)
        self.bq_client = bigquery.Client(credentials=credentials)
        self.pubsub_publisher = pubsub_v1.PublisherClient(credentials=credentials)
        self.firestore_client = cloud.firestore.Client(credentials=credentials)
        self.secret_client = secretmanager.SecretManagerServiceClient(credentials=credentials)
        self.translate_client = translatergcp.Client(credentials=credentials)

    def get_file_url(self, bucket_name, blob_name):
        bucket = self.storage_client.bucket(bucket_name)
        blob = bucket.blob(blob_name)
        return blob.public_url

    def run_bq_query(self, query):
        query_job = self.bq_client.query(query)
        return [dict(row.items()) for row in query_job.result()]

    def publish_message(self, topic_name, message):
        topic_path = self.pubsub_publisher.topic_path(self.config['project_id'], topic_name)
        future = self.pubsub_publisher.publish(topic_path, message.encode("utf-8"))
        return future.result()

    def add_firestore_document(self, collection, doc_id, data):
        doc_ref = self.firestore_client.collection(collection).document(doc_id)
        doc_ref.set(data)
        return f"Document {doc_id} added to {collection}"

    def get_secret(self, secret_id, version="latest"):
        name = f"projects/{self.config['project_id']}/secrets/{secret_id}/versions/{version}"
        response = self.secret_client.access_secret_version(request={"name": name})
        return response.payload.data.decode("UTF-8")

    def translate_text(self, text, target="en"):
        result = self.translate_client.translate(text, target_language=target)
        return result['translatedText']


# Flask App
app = Flask(__name__)
config = {
    "project_id": os.environ.get("GCP_PROJECT_ID", "your-gcp-project-id")
}
gcp_services = GCPServices(config)


@app.route("/storage/url", methods=["GET"])
def get_file_url():
    bucket = request.args.get("bucket")
    blob = request.args.get("blob")
    url = gcp_services.get_file_url(bucket, blob)
    return jsonify({"url": url})


@app.route("/bigquery/query", methods=["POST"])
def bigquery_query():
    query = request.json.get("query")
    result = gcp_services.run_bq_query(query)
    return jsonify(result)


@app.route("/pubsub/publish", methods=["POST"])
def pubsub_publish():
    topic = request.json.get("topic")
    message = request.json.get("message")
    msg_id = gcp_services.publish_message(topic, message)
    return jsonify({"message_id": msg_id})


@app.route("/firestore/add", methods=["POST"])
def firestore_add():
    collection = request.json.get("collection")
    doc_id = request.json.get("doc_id")
    data = request.json.get("data")
    status = gcp_services.add_firestore_document(collection, doc_id, data)
    return jsonify({"status": status})


@app.route("/secret/get", methods=["GET"])
def secret_get():
    secret_id = request.args.get("secret_id")
    version = request.args.get("version", "latest")
    secret = gcp_services.get_secret(secret_id, version)
    return jsonify({"secret": secret})


@app.route("/translate", methods=["POST"])
def translate_text():
    text = request.json.get("text")
    target = request.json.get("target", "en")
    translated = gcp_services.translate_text(text, target)
    return jsonify({"translated": translated})


if __name__ == "__main__":
    app.run(debug=True)
