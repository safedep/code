import static java.util.stream.Collectors.toList;

import com.openai.client.OpenAIClient;
import com.openai.client.okhttp.OpenAIOkHttpClient;
import com.openai.models.audio.AudioModel;
import com.openai.models.audio.transcriptions.Transcription;
import com.openai.models.audio.transcriptions.TranscriptionCreateParams;
import com.azure.identity.AuthenticationUtil;
import com.azure.identity.DefaultAzureCredentialBuilder;
import com.openai.credential.BearerTokenCredential;
import com.openai.models.ChatModel;
import com.openai.models.chat.completions.ChatCompletionCreateParams;
import com.openai.models.embeddings.EmbeddingCreateParams;
import com.openai.models.embeddings.EmbeddingModel;
import com.openai.models.chat.completions.ChatCompletionCreateParams;
import com.openai.models.chat.completions.ChatCompletionMessage;
import java.util.List;
import com.openai.models.images.ImageGenerateParams;
import com.openai.models.images.ImageModel;
import java.util.concurrent.CompletableFuture;
import com.azure.ai.openai.OpenAIClient;
import com.azure.ai.openai.OpenAIClientBuilder;
import com.azure.ai.openai.models.Choice;
import com.azure.ai.openai.models.Completions;
import com.azure.ai.openai.models.CompletionsOptions;
import com.azure.ai.openai.models.CompletionsUsage;
import com.azure.core.credential.AzureKeyCredential;
import com.azure.core.util.Configuration;

import java.util.ArrayList;
import java.util.List;

public final class OpenAiUsage {
    public static void main(String[] args) throws Exception {
        // Configures using one of:
        // - The `OPENAI_API_KEY` environment variable
        // - The `OPENAI_BASE_URL` and `AZURE_OPENAI_KEY` environment variables
        OpenAIClient client = OpenAIOkHttpClient.fromEnv();
        OpenAIClient client2 = OpenAIOkHttpClient.builder()
            // Gets the API key from the `AZURE_OPENAI_KEY` environment variable
            .fromEnv()
            // Set the Azure Entra ID
            .credential(BearerTokenCredential.create(AuthenticationUtil.getBearerTokenSupplier(
                    new DefaultAzureCredentialBuilder().build(), "https://cognitiveservices.azure.com/.default")))
            .build();
        
        // Audio
        TranscriptionCreateParams createParams = TranscriptionCreateParams.builder()
                .file("path")
                .model(AudioModel.WHISPER_1)
                .build();
        Transcription transcription =
                client.audio().transcriptions().create(createParams).asTranscription();
        System.out.println(transcription.text());

        // Chat
        ChatCompletionCreateParams chatParams = ChatCompletionCreateParams.builder()
                .model(ChatModel.GPT_3_5_TURBO)
                .maxCompletionTokens(2048)
                .addDeveloperMessage("Make sure you mention Stainless!")
                .addUserMessage("Tell me a story about building the best SDK!")
                .build();
        client.chat()
                .completions()
                .create(chatParams)
                .thenAccept(completion -> completion.choices().stream()
                        .flatMap(choice -> choice.message().content().stream())
                        .forEach(System.out::println))
                .join();
        
        // Completions conversation async
        ChatCompletionCreateParams.Builder createParamsBuilder = ChatCompletionCreateParams.builder()
                .model(ChatModel.GPT_3_5_TURBO)
                .maxCompletionTokens(2048)
                .addDeveloperMessage("Make sure you mention Stainless!")
                .addUserMessage("Tell me a story about building the best SDK!");

        CompletableFuture<Void> future = CompletableFuture.completedFuture(null);
        for (int i = 0; i < 4; i++) {
            final int index = i;
            future = future.thenComposeAsync(
                            unused -> client.chat().completions().create(createParamsBuilder.build()))
                    .thenAccept(completion -> {
                        List<ChatCompletionMessage> messages = completion.choices().stream()
                                .map(ChatCompletion.Choice::message)
                                .collect(toList());

                        messages.stream()
                                .flatMap(message -> message.content().stream())
                                .forEach(System.out::println);

                        System.out.println("\n-----------------------------------\n");

                        messages.forEach(createParamsBuilder::addMessage);
                        createParamsBuilder
                                .addDeveloperMessage("Be as snarky as possible when replying!" + "!".repeat(index))
                                .addUserMessage("But why?" + "?".repeat(index));
                    });
        }
        future.join();

        // Embeddings
        EmbeddingCreateParams embeddingParams = EmbeddingCreateParams.builder()
                .input("The quick brown fox jumped over the lazy dog")
                .model(EmbeddingModel.TEXT_EMBEDDING_3_SMALL)
                .build();
        client.embeddings().create(embeddingParams).thenAccept(System.out::println).join();
    

        ImageGenerateParams imageGenerateParams = ImageGenerateParams.builder()
                .responseFormat(ImageGenerateParams.ResponseFormat.URL)
                .prompt("Two cats playing ping-pong")
                .model(ImageModel.DALL_E_2)
                .size(ImageGenerateParams.Size._512X512)
                .n(1)
                .build();
        client.images().generate(imageGenerateParams).data().orElseThrow().stream()
                .flatMap(image -> image.url().stream())
                .forEach(System.out::println);
    }
}

// Openai azure
public class OpenAiAzureUsage {
    /**
     * Runs the sample algorithm and demonstrates how to get completions for the provided input prompts.
     * Completions support a wide variety of tasks and generate text that continues from or "completes" provided
     * prompt data.
     *
     * @param args Unused. Arguments to the program.
     */
    public static void main(String[] args) {
        String azureOpenaiKey = Configuration.getGlobalConfiguration().get("AZURE_OPENAI_KEY");
        String endpoint = Configuration.getGlobalConfiguration().get("AZURE_OPENAI_ENDPOINT");
        String deploymentOrModelId = "{azure-open-ai-deployment-model-id}";

        OpenAIClient client = new OpenAIClientBuilder()
            .endpoint(endpoint)
            .credential(new AzureKeyCredential(azureOpenaiKey))
            .buildClient();

        List<String> prompt = new ArrayList<>();
        prompt.add("Why did the eagles not carry Frodo Baggins to Mordor?");

        Completions completions = client.getCompletions(deploymentOrModelId, new CompletionsOptions(prompt));

        System.out.printf("Model ID=%s is created at %s.%n", completions.getId(), completions.getCreatedAt());
        for (Choice choice : completions.getChoices()) {
            System.out.printf("Index: %d, Text: %s.%n", choice.getIndex(), choice.getText());
        }

        CompletionsUsage usage = completions.getUsage();
        System.out.printf("Usage: number of prompt token is %d, "
                + "number of completion token is %d, and number of total tokens in request and response is %d.%n",
            usage.getPromptTokens(), usage.getCompletionTokens(), usage.getTotalTokens());
    }
}