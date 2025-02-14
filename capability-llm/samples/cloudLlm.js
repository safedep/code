// AWS S3 Operations
const AWS = require('aws-sdk');
const { Storage } = require('@google-cloud/storage');
const { TextAnalyticsClient, AzureKeyCredential } = require('@azure/ai-text-analytics');
const openai = require('openai');

openai.apiKey = process.env.OPENAI_API_KEY;

const s3 = new AWS.S3();
const gcs = new Storage();

const uploadToS3 = async () => {
    const params = {
        Bucket: 'my-bucket',
        Key: 'file.txt',
        Body: 'Hello from AWS S3!'
    };

    try {
        await s3.putObject(params).promise();
        console.log('File uploaded to S3');
    } catch (err) {
        console.error('Error uploading to S3:', err);
    }
};

const uploadToGCS = async () => {
    const bucketName = 'my-gcp-bucket';
    const filename = 'file.txt';

    try {
        await gcs.bucket(bucketName).upload(filename);
        console.log('File uploaded to Google Cloud Storage');
    } catch (err) {
        console.error('Error uploading to GCS:', err);
    }
};

const endpoint = 'https://my-text-analytics.cognitiveservices.azure.com/';
const apiKey = 'YOUR_AZURE_API_KEY';
const client = new TextAnalyticsClient(endpoint, new AzureKeyCredential(apiKey));

const analyzeText = async () => {
    const documents = ["I had a wonderful experience at the hotel. The staff was helpful and the room was clean."];

    try {
        const results = await client.analyzeSentiment(documents);
        results.forEach(result => {
            console.log(`Document sentiment: ${result.sentiment}`);
        });
    } catch (err) {
        console.error('Error analyzing text:', err);
    }
};

const runLLM = async () => {
    try {
        const completion = await openai.Completion.create({
            model: "text-davinci-003",
            prompt: "Explain the benefits of cloud computing.",
            max_tokens: 100
        });
        console.log('OpenAI Response:', completion.choices[0].text);
    } catch (err) {
        console.error('Error with OpenAI API:', err);
    }
};

(async () => {
    await uploadToS3();
    await uploadToGCS();
    await analyzeText();
    await runLLM();
})();
