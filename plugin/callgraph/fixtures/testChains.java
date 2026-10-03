import io.pinecone.clients.Pinecone;
import com.example.http.Request;

class TestChains {
    void run() {
        new Pinecone.Builder("key").build();
        new Request("https://example.com").send().join();
    }
}
