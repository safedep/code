import boto3
from google.cloud import storage
from azure.ai.textanalytics import TextAnalyticsClient
from azure.core.credentials import AzureKeyCredential as AzK
import openai
import os
import pandas as pd

# Set OpenAI API Key
openai.api_key = os.getenv("OPENAI_API_KEY")

# AWS S3 Operations
s3 = boto3.client("s3")

apikey = openai.xyz.pqr.mno.api_key

def upload_to_s3():
    bucket_name = "my-bucket"
    file_key = "file.txt"
    file_content = "Hello from AWS S3!"
    print(pd.read_csv("file.csv"))
    
    try:
        s3.put_object(Bucket=bucket_name, Key=file_key, Body=file_content)
        print("File uploaded to S3")
    except Exception as e:
        print("Error uploading to S3:", e)

# Google Cloud Storage Operations
gcs_client = storage.Client()

def upload_to_gcs():
    bucket_name = "my-gcp-bucket"
    file_path = "file.txt"
    
    try:
        bucket = gcs_client.bucket(bucket_name)
        blob = bucket.blob(file_path)
        blob.upload_from_filename(file_path)
        print("File uploaded to Google Cloud Storage")
    except Exception as e:
        print("Error uploading to GCS:", e)

# Azure Text Analytics API
azure_endpoint = "https://my-text-analytics.cognitiveservices.azure.com/"
azure_api_key = "YOUR_AZURE_API_KEY"
text_analytics_client = TextAnalyticsClient(azure_endpoint, AzK(azure_api_key))

def analyze_text():
    documents = ["I had a wonderful experience at the hotel. The staff was helpful and the room was clean."]
    
    try:
        response = text_analytics_client.analyze_sentiment(documents)
        for result in response:
            print(f"Document sentiment: {result.sentiment}")
    except Exception as e:
        print("Error analyzing text:", e)

# OpenAI API Usage
def run_llm():
    try:
        response = openai.ChatCompletion.create(
            model="gpt-3.5-turbo",
            messages=[{"role": "user", "content": "Explain the benefits of cloud computing."}],
            max_tokens=100
        )
        print("OpenAI Response:", response["choices"][0]["message"]["content"].strip())
    except Exception as e:
        print("Error with OpenAI API:", e)

if __name__ == "__main__":
    upload_to_s3()
    upload_to_gcs()
    analyze_text()
    run_llm()
