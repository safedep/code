import os
from flask import Flask, request, jsonify
from azure.storage.blob import BlobServiceClient
from azure.cosmos import CosmosClient
from azure.keyvault.secrets import SecretClient
from azure.ai.translation.text import TextTranslationClient, TranslatorCredential
from azure.servicebus import ServiceBusClient, ServiceBusMessage
from azure.identity import DefaultAzureCredential

class BaseAzureService:
    # Note - this block is unreachabel in DFS, as parent constructors aren't resolved yet
    def __init__(self, config: dict):
        # Use DefaultAzureCredential which supports multiple authentication methods
        self.credential = DefaultAzureCredential() 
        self.config = config

class AzureStorageServices(BaseAzureService):
    def __init__(self, config: dict):
        super().__init__(config)
        storage_connection_string = os.environ.get("AZURE_STORAGE_CONNECTION_STRING")
        if not storage_connection_string:
            raise EnvironmentError("AZURE_STORAGE_CONNECTION_STRING env var not set")
        
        self.blob_service_client = BlobServiceClient.from_connection_string(storage_connection_string)
        self.cosmos_client = CosmosClient(
            url=config["cosmos_endpoint"], 
            credential=self.credential
        )
        self.keyvault_client = SecretClient(
            vault_url=f"https://{config['keyvault_name']}.vault.azure.net/", 
            credential=self.credential
        )
        
    def get_file_url(self, container_name, blob_name):
        blob_client = self.blob_service_client.get_blob_client(
            container=container_name, 
            blob=blob_name
        )
        return blob_client.url
    
    def run_cosmos_query(self, database_name, container_name, query):
        database = self.cosmos_client.get_database_client(database_name)
        container = database.get_container_client(container_name)
        items = list(container.query_items(query=query, enable_cross_partition_query=True))
        return items
    
    def add_cosmos_document(self, database_name, container_name, data):
        database = self.cosmos_client.get_database_client(database_name)
        container = database.get_container_client(container_name)
        response = container.create_item(body=data)
        return f"Document {response['id']} added to {container_name}"
    
    def get_secret(self, secret_name):
        secret = self.keyvault_client.get_secret(secret_name)
        return secret.value

class AzureAiServices(BaseAzureService):
    def __init__(self, config: dict):
        super().__init__(config)
        translator_key = os.environ.get("AZURE_TRANSLATOR_KEY")
        if not translator_key:
            raise EnvironmentError("AZURE_TRANSLATOR_KEY env var not set")
            
        self.translator_credential = TranslatorCredential(translator_key, config["translator_region"])
        self.translator_client = TextTranslationClient(credential=self.translator_credential)
    
    def translate_text(self, text, target="en"):
        response = self.translator_client.translate(
            content=[text],
            to=[target]
        )
        return response[0].translations[0].text

class AzureMessagingServices(BaseAzureService):
    def __init__(self, config: dict):
        super().__init__(config)
        self.servicebus_connection_string = os.environ.get("AZURE_SERVICEBUS_CONNECTION_STRING")
        self.client = ServiceBusClient.from_connection_string(
            conn_str=self.servicebus_connection_string
        )
        if not self.servicebus_connection_string:
            raise EnvironmentError("AZURE_SERVICEBUS_CONNECTION_STRING env var not set")
    
    def publish_message(self, queue_name, message):
            with self.client.get_queue_sender(queue_name) as sender:
                message = ServiceBusMessage(message)
                sender.send_messages(message)
                return "Message sent successfully"

# Flask App
app = Flask(__name__)
config = {
    "cosmos_endpoint": os.environ.get("AZURE_COSMOS_ENDPOINT", "https://your-cosmos-account.documents.azure.com:443/"),
    "keyvault_name": os.environ.get("AZURE_KEYVAULT_NAME", "your-keyvault-name"),
    "translator_region": os.environ.get("AZURE_TRANSLATOR_REGION", "eastus")
}

storage_services = AzureStorageServices(config)
ai_services = AzureAiServices(config)
messaging_services = AzureMessagingServices(config)

@app.route("/storage/url", methods=["GET"])
def get_file_url():
    container = request.args.get("container")
    blob = request.args.get("blob")
    url = storage_services.get_file_url(container, blob)
    return jsonify({"url": url})

@app.route("/cosmos/query", methods=["POST"])
def cosmos_query():
    database = request.json.get("database")
    container = request.json.get("container")
    query = request.json.get("query")
    result = storage_services.run_cosmos_query(database, container, query)
    return jsonify(result)

@app.route("/servicebus/publish", methods=["POST"])
def servicebus_publish():
    queue = request.json.get("queue")
    message = request.json.get("message")
    status = messaging_services.publish_message(queue, message)
    return jsonify({"status": status})

@app.route("/cosmos/add", methods=["POST"])
def cosmos_add():
    database = request.json.get("database")
    container = request.json.get("container")
    data = request.json.get("data")
    status = storage_services.add_cosmos_document(database, container, data)
    return jsonify({"status": status})

@app.route("/secret/get", methods=["GET"])
def secret_get():
    secret_name = request.args.get("secret_name")
    secret = storage_services.get_secret(secret_name)
    return jsonify({"secret": secret})

@app.route("/translate", methods=["POST"])
def translate_text():
    text = request.json.get("text")
    target = request.json.get("target", "en")
    translated = ai_services.translate_text(text, target)
    return jsonify({"translated": translated})

if __name__ == "__main__":
    app.run(debug=True)