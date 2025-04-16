import boto3
from botocore.exceptions import NoCredentialsError, ClientError
from datetime import datetime

# --- S3: Upload a file to a bucket ---
def upload_to_s3_basic(bucket_name, file_name, object_name=None):
    s3 = boto3.client('s3')
    try:
        s3.upload_file(file_name, bucket_name, object_name or file_name)
        print(f"Uploaded {file_name} to S3 bucket {bucket_name}")
        return object_name or file_name
    except FileNotFoundError:
        print("The file was not found")
    except NoCredentialsError:
        print("AWS credentials not found")

# --- S3: Upload a file to a bucket ---
def upload_to_s3(bucket_name, file_name, object_name=None):
    # s3 = boto3.client('s3')
    customResource = 's3'
    s3 = boto3.resource(customResource)
    try:
        s3.upload_file(file_name, bucket_name, object_name or file_name)
        print(f"Uploaded {file_name} to S3 bucket {bucket_name}")
        return object_name or file_name
    except FileNotFoundError:
        print("The file was not found")
    except NoCredentialsError:
        print("AWS credentials not found")

# --- EC2: List all EC2 instances ---
def list_ec2_instances():
    ec2 = boto3.client('ec2')
    response = ec2.describe_instances()
    print("Listing EC2 instances:")
    for reservation in response['Reservations']:
        for instance in reservation['Instances']:
            print(f"ID: {instance['InstanceId']}, State: {instance['State']['Name']}")

# --- EC2: Start an EC2 instance ---
def start_ec2_instance(instance_id):
    ec2 = boto3.client('ec2')
    try:
        response = ec2.start_instances(InstanceIds=[instance_id])
        print(f"Starting instance {instance_id}... current state: {response['StartingInstances'][0]['CurrentState']['Name']}")
    except ClientError as e:
        print(f"Error starting instance: {e}")

# --- SQS: Send a message to queue ---
def send_sqs_message(queue_url, message_body):
    sqs = boto3.client('sqs')
    response = sqs.send_message(QueueUrl=queue_url, MessageBody=message_body)
    print(f"Sent message to SQS: {response['MessageId']}")

# --- SQS: Receive a message ---
def receive_sqs_message(queue_url):
    sqs = boto3.client('sqs')
    messages = sqs.receive_message(QueueUrl=queue_url, MaxNumberOfMessages=1)
    if 'Messages' in messages:
        for msg in messages['Messages']:
            print(f"Received message: {msg['Body']}")
            sqs.delete_message(QueueUrl=queue_url, ReceiptHandle=msg['ReceiptHandle'])
    else:
        print("No messages in queue.")

# --- SNS: Publish notification ---
def publish_sns_message(topic_arn, subject, message):
    sns = boto3.client('sns')
    response = sns.publish(TopicArn=topic_arn, Subject=subject, Message=message)
    print(f"Published SNS message: {response['MessageId']}")

# --- DynamoDB: Store metadata of uploaded file ---
def log_upload_metadata(table_name, file_name, bucket):
    dynamodb = boto3.resource('dynamodb')
    table = dynamodb.Table(table_name)
    table.put_item(Item={
        'FileName': file_name,
        'Bucket': bucket,
        'Timestamp': datetime.utcnow().isoformat()
    })
    print(f"Logged metadata to DynamoDB for file: {file_name}")

# --- MAIN FUNCTION ---
if __name__ == "__main__":
    # S3
    bucket_name = 'your-bucket-name'
    file_name = 'example.txt'
    uploaded_file = upload_to_s3(bucket_name, file_name)

    # Log metadata to DynamoDB
    log_upload_metadata('your-dynamodb-table', uploaded_file, bucket_name)

    # EC2
    list_ec2_instances()
    start_ec2_instance('i-0123456789abcdef0')

    # SQS
    send_sqs_message('https://sqs.us-east-1.amazonaws.com/123456789012/your-queue-name', 'Hello from Python!')
    receive_sqs_message('https://sqs.us-east-1.amazonaws.com/123456789012/your-queue-name')

    # SNS
    publish_sns_message('arn:aws:sns:us-east-1:123456789012:your-topic-name', 'Upload Alert', f'File {uploaded_file} uploaded to {bucket_name}')
