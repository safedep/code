from pydantic import BaseModel
from fastapi import Form
import datetime

class Task(BaseModel):
    id: str = Form(default="")
    description: str = Form(default="")
    title: str = Form(default="")
    assigned_to: list = Form(default=[])
    image: str = Form(default="")
    last_modified: str = Form(default=datetime.datetime.now())
    created_at: str = Form(default=datetime.datetime.now())
    deadline: str = Form(default="")
    status: str = Form(default="Unassigned")
    #status are: Unassigned, Assigned, Review, Completed
    assc_incident: str = Form(default="N/A")
    dept_name: str = Form(default="")
    station_name: str = Form(default="")
    
class IncidentToTask(BaseModel):
    incident_id: str = Form(...)
    deadline: str = Form(...)
    assigned_to: list = Form(default=[])