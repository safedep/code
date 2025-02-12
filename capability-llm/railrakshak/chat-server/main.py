import os
from dotenv import load_dotenv
from enum import Enum
from pyngrok import ngrok
from flask import Flask, request
from twilio.twiml.messaging_response import MessagingResponse
from utils import *

load_dotenv()

PORT = os.getenv('FLASK_RUN_PORT') or 5002
NGROK_AUTHTOKEN = os.getenv('NGROK_AUTHTOKEN')
NGROK_DOMAIN = os.environ.get('NGROK_DOMAIN')

app = Flask(__name__)
ngrok.set_auth_token(NGROK_AUTHTOKEN)
tunnel_url = ngrok.connect(
    PORT, bind_tls=True, hostname=NGROK_DOMAIN).public_url

print(f"Ingress established at {tunnel_url}")


class Stages(Enum):
    INIT = 1
    LOCATION = 2
    LOCATION2 = 3
    MEDIA = 4


curr_stage = Stages.INIT


@app.route("/whatsapp", methods=["GET", "POST"])
def reply_whatsapp():
    print(request.values)
    request.values.get("Body")
    global curr_stage
    incoming_message_body = request.values.get("Body")
    incoming_message = (incoming_message_body or "").strip().lower()
    # num_media = int(request.values.get("NumMedia"))
    print("Incoming message: ", incoming_message)

    response = MessagingResponse()
    modified_state = False

    if contains_initiating_strings(incoming_message):
        msg = response.message(
            "Welcome to Rail Rakshak !\nWhat are you looking for ?\n1. Report an incident\n2. Helpdesk", quick_replies=[
                "1. Report an incident",
                "2. Helpdesk",
            ])

    elif incoming_message == "1" or is_report_incident_input(incoming_message):
        msg = response.message(
            "Sure, please describe your incident")
    elif incoming_message == "2" or is_helpdesk_input(incoming_message):
        msg = response.message(
            "Here are the helpline phone numbers:\nRailway Police: 1800 1113 22 \nVigilance Helpline: 0111 552 10")
    elif is_conclusive(incoming_message):
        msg = response.message(
            "If you need further assistance, feel free to reach out anytime.\nBye 👋")
    else:
        incident_id = generate_alphanumeric_id()
        if curr_stage == Stages.INIT:
            curr_stage = Stages.LOCATION
            msg = response.message(
                "On which railway station did you witness this ?")
        elif curr_stage == Stages.LOCATION:
            msg = response.message("Can you tell us the platform number ?")
            curr_stage = Stages.LOCATION2
        elif curr_stage == Stages.LOCATION2:
            msg = response.message(
                "Have you captured any image/video of the incident ?")
            curr_stage = Stages.MEDIA
        else:
            msg = response.message(
                f"Thank you for reporting the incident !\nReference ID: {incident_id}")
            curr_stage = Stages.INIT
        modified_state = True

    if not modified_state:
        curr_stage = Stages.INIT
    print("Formatted xml reply: \n", msg)
    return str(response)


@app.route("/ping", methods=["GET", "POST"])
def ping():
    return f"GG !!"

@app.route("/stage", methods=["GET", "POST"])
def stage():
    return str(curr_stage.name)

if __name__ == "__main__":
    app.run()
