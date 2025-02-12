from fastapi import APIRouter
from fastapi import UploadFile, Form
from src.models.incidents_model import Incidents
from src.models.notifications_model import Notifications
from src.database.notifications_db import create_notification
from src.database.incident_db import (create_incident, fetch_all_incidents, fetch_incidents_by_dept_and_station, delete_incident_by_id)
from src.config import AWS_KEY, SECRET_KEY_AWS, S3_BUCKET_NAME
import boto3
import random
from ultralytics import YOLO
import cv2
import numpy as np
import io

s3 = boto3.resource(
    service_name='s3',
    aws_access_key_id=f"{AWS_KEY}",
    aws_secret_access_key=f"{SECRET_KEY_AWS}"
)
bucket = s3.Bucket(S3_BUCKET_NAME)

model = YOLO("./assets/last.pt")

router = APIRouter(
    prefix="/incidents",
    tags=["Incidents"],
    responses={404: {"description": "Not found"}},
)

# function that generates random id of length 8
def generateID():
    id = ""
    for i in range(8):
        if random.random() < 0.5:
            id += chr(random.randint(65,90))
        else:
            id += str(random.randint(0,9))
    return id

@router.post("/create_incident")
def new_incident(incident: Incidents):
    try:
        if incident.title == "" or incident.type == "" or incident.station_name == "" or incident.source == "" or incident.image == "":
            return {"ERROR": "MISSING PARAMETERS"}

        result = create_incident(incident)
        return result
    except Exception as e:
        print(e)
        return {"ERROR":"SOME ERROR OCCURRED"}

@router.post("/user_incident")
async def create_incident_by_user(image: UploadFile, title: str = Form(...), description: str = Form(...), type: str = Form(...), station_name: str = Form(...), location: str = Form(...), source: str = Form(...)):
    try:
        filename = image.filename.replace(" ","")
        img_extension = filename.split(".")[1]
            
        if img_extension not in ["png", "jpg","jpeg"]:
            return {"ERROR":"INVALID IMAGE FORMAT"}

        # Read the image file into a numpy array
        contents = await image.read()
        nparr = np.fromstring(contents, np.uint8)

        # Decode the image using OpenCV
        img = cv2.imdecode(nparr, cv2.IMREAD_COLOR)
        results = model.predict(source=img, save_txt=False)
        img_with_boxes = results[0].plot()

        # Save the image with boxes to a BytesIO object
        _, im_buf_arr = cv2.imencode(".jpg", img_with_boxes)
        byte_im = im_buf_arr.tobytes()

        # Create a BytesIO object from the byte array
        byte_im_io = io.BytesIO(byte_im)

        incident = Incidents(title=title, description=description, type=type, station_name=station_name, location=location, source=source)

        uname = str(filename.split(".")[0] + generateID() + ".jpg")
        bucket.upload_fileobj(byte_im_io, uname)
        url = f"https://{S3_BUCKET_NAME}.s3.amazonaws.com/{uname}"
        incident.image = url

        result = create_incident(incident)
        # create notification
        if incident.type in ["Safety Threat", "Violence", "Stampede", "Crime"]:
            notification = Notifications(station_name=incident.station_name, dept_name="Security", title="New Report: " + incident.title, description=incident.description, type="report")
            create_notification(notification)
        return result
    except Exception as e:
        print(e)
        return {"ERROR":"SOME ERROR OCCURRED"}

## get all incidents
@router.get("/all_incidents")
def get_all_incidents():
    return fetch_all_incidents()
    
## get incident by dept name and station name
@router.get("/get_incidents_by_dept_and_station")
def get_incidents_by_dept_and_station(dept_name: str, station_name: str):
    return fetch_incidents_by_dept_and_station(dept_name, station_name)

@router.delete("/delete_incident")
def delete_incident(id: str):
    return delete_incident_by_id(id)