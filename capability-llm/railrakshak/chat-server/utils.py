import random
import string


def generate_alphanumeric_id(length=6):
    # Includes both letters (uppercase and lowercase) and digits
    characters = string.ascii_uppercase + string.digits
    return ''.join(random.choice(characters) for _ in range(length))


# List of initiating strings
initiating_strings = ["hi", "hello", "hey", "howdy", "hi there", "hiya", "hiya there",
                      "greetings", "good day", "yo", "hi folks", "hi chatbot", "hi rail rakshak", "hi railrakshak", "namaste"]


def contains_initiating_strings(input_string):
    # Convert the input string to lowercase for case-insensitive comparison
    lowercase_input = input_string.lower()

    # Check if any of the initiating strings are present in the lowercase input
    for init_str in initiating_strings:
        if init_str.lower() in lowercase_input:
            return True

    return False


# Define a list of possible inputs for reporting an incident
report_incident_inputs = [
    "report",
    "report incident",
    "wish to report",
    "want to report",
    "incident report",
    "reporting an incident",
    "report this incident",
    "incident",
    "incident reporting",
    "report issue",
    "issue report",
    "emergency",
    "emergency report",
    "urgent incident",
    "urgent report",
    "safety concern",
    "concern report",
    "problem report",
]


def is_report_incident_input(message):
    # Check if the message is in the list of possible inputs
    return any(input_text in message for input_text in report_incident_inputs)


# Define a list of possible inputs for selecting the helpdesk option
helpdesk_inputs = [
    "helpdesk",
    "help",
    "support",
    "need assistance",
    "helpline",
    "customer support",
    "contact support",
    "get help",
    "assistance",
    "help needed",
    "support hotline",
]


def is_helpdesk_input(message):
    # Check if the message is in the list of possible inputs
    return any(input_text in message for input_text in helpdesk_inputs)


# Define a list of possible expressions of gratitude
thank_you_inputs = ["thanks", "thank you",
                    "thanks a lot", "thank u", "thx", "bye", "see u", "see you"]


def is_conclusive(message):
    # Check if the message is in the list of possible thank you expressions
    return any(input_text in message for input_text in thank_you_inputs)

# # Test cases
# test_cases = [
#     "Hi there, how are you?",
#     "Hello, world!",
#     "Hey, everyone!",
#     "Howdy, folks!",
#     "Yo, mate!",
#     "Greetings, Earthlings!",
#     "Good day to you!",
#     "This is a test.",
#     "No greetings here.",
#     "Hey there, howdy folks?",
#     "Hi friends, it's good to see you!",
#     "What's up, hiya there?",
#     "Hi all, this is a friendly message.",
#     "Hello, how's it going?",
# ]

# # Test the strings
# for test_string in test_cases:
#     if contains_initiating_strings(test_string, initiating_strings):
#         print(f"Yes - The string '{test_string}' contains one of the initiating strings.")
#     else:
#         print(f"No - The string '{test_string}' does not contain any of the initiating strings.")
