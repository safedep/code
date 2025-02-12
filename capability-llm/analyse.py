import os
import json
import re
import time
from openai import AzureOpenAI

azure_openai_key = os.getenv("OPENAI_API_KEY")
azure_openai_endpoint = os.getenv("OPENAI_BASE_URL")

if azure_openai_key == None:
    raise ValueError("API key is not set. Please set the OPENAI_API_KEY environment variable.")
if azure_openai_endpoint == None:
    raise ValueError("API endpoint is not set. Please set the OPENAI_BASE_URL environment variable.")

client = AzureOpenAI(
  azure_endpoint = azure_openai_endpoint,
  api_key=azure_openai_key,
  api_version="2024-02-01"
)

with open('capabilities.txt', 'r') as file:
    knownCapabilities = file.read()
    knownCapabilities = knownCapabilities.strip()

def analyze_code(file_path, output_path):
    with open(file_path, 'r') as file:
        code_content = file.read()
    code_content = code_content.strip()

    if len(code_content) > 1000:
      code_content = code_content[:1000]

    if len(code_content) == 0:
      emptyJson = """{"capabilities": []}"""
      with open(output_path, 'w') as outfile:
        json.dump(json.loads(emptyJson), outfile, indent=4)
      return

    prompt = f"""Analyze the following code and identify program dependencies and capabilities. Provide the results in JSON format, including capability IDs and evidence details (line numbers and corresponding code snippets).


Code:
{code_content}

Some example capabilities, you may also add more capabilities if required which follow the same format:
{knownCapabilities}

Provide the results **strictly in JSON format**, without any additional explanation or text and only with capabilities that are strongly identified in the code. Ensure that the JSON format is valid and characters like "" are properly escaped.

Return the output **only** in the following JSON format:
{{
  'capabilities': [
    {{
      'capability_id': "network:http",
      'evidence': [
        {{
          "line_no": 55,
          "line_content": "response = requests.get(\"https://api.example.com/data\")"
        }}
      ]
    }},
    ...
  ]
}}"""

    print("Request prompt for", file_path, ":", prompt)

    response = client.chat.completions.create(
        model="gpt-4o",
        messages=[
            {"role": "system", "content": "You are a code analysis expert."},
            {"role": "user", "content": prompt}
        ]
    )
    print(response)
    print("Response content", response.choices[0].message.content)

    # Extract and store the result
    result = response.choices[0].message.content
    
     # Remove ```json and ``` if present
    cleaned_result = re.sub(r'^```json\s*|\s*```$', '', result.strip(), flags=re.DOTALL)

    # print("cleaned", cleaned_result)

    with open(output_path, 'w') as outfile:
        json.dump(json.loads(cleaned_result), outfile, indent=4)


mergeOnly = False
knownExtensions = ["py", "js", "go", "ts", "tsx", "mjs"]

scanFolders = []
# scanFolders.append("samples")
# scanFolders.append("railrakshak")
# scanFolders.append("prettier")
scanFolders.append("scancode-workbench")

os.makedirs("results", exist_ok=True)


try:
  # Scan directories and analyze code
  if mergeOnly:
    raise Exception("Only merging results")
  for folder in scanFolders:
      for root, _, files in os.walk(folder):
          for file in files:
              if file.split('.')[-1] in knownExtensions:
                  file_path = os.path.join(root, file)
                  
                  # Construct output path
                  relative_path = os.path.relpath(file_path, folder)
                  output_folder = f"results/{folder}-results"
                  output_path = os.path.join(output_folder, f"{relative_path}.json")
                  os.makedirs(os.path.dirname(output_path), exist_ok=True)

                  # Analyze and save result
                  print(f"Start analyzing {file_path} -> {output_path}")
                  analyze_code(file_path, output_path)
                  print(f"Analyzed {file_path} -> {output_path}")

                  # time.sleep(2)
except Exception as e:
  print("Error gathering results", e)
finally:
  print("Start merging ...")
  merged_results = {"files": {}}
  # Travese all json files in the results folder and merge them
  resultsFolder = "results"
  mergedFileName = "merged_results.json"
  resultsJson = "results/" + mergedFileName
  for root, _, files in os.walk(resultsFolder):
    for file in files:
      if file == mergedFileName:
        continue
      if file.endswith(".json"):
        file_path = os.path.join(root, file)
        print("merge ", file_path)
        with open(file_path, 'r') as f:
          try:
            content = json.load(f)
            relative_file_path = os.path.relpath(file_path, resultsFolder).replace(".json", "")
            merged_results["files"][relative_file_path] = content
          except Exception as e:
            print("Error loading", file_path, e)
  # for folder in scanFolders:
  #     result_folder = f"results/{folder}-results"
  #     for root, _, files in os.walk(result_folder):
  #         for file in files:
  #             if file.endswith(".json"):
  #                 file_path = os.path.join(root, file)
  #                 with open(file_path, 'r') as f:
  #                     content = json.load(f)
                  
  #                 relative_file_path = os.path.relpath(file_path, result_folder).replace(".json", "")
  #                 merged_results["files"][f"{folder}/{relative_file_path}"] = content
  print("Storing merged ...")

  # erase previous merged results
  if os.path.exists(resultsJson):
    os.remove(resultsJson)
    print("Erased previous merged results.")
  with open(resultsJson, 'w') as merged_file:
      json.dump(merged_results, merged_file, indent=4)
  print("Stored merged results.")

