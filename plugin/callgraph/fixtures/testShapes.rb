require 'openai'
require 'net/http'

class Assistant
  def initialize
    @client = OpenAI::Client.new(access_token: ENV.fetch("OPENAI_API_KEY"))
  end

  def ask(prompt)
    client = OpenAI::Client.new
    response = client.chat(parameters: { model: "gpt-4o", messages: [{ role: "user", content: prompt }] })
    [1, 2].each do |i|
      Net::HTTP.get(URI("https://example.com/#{i}"))
    end
    response.dig("choices", 0)
  end
end
