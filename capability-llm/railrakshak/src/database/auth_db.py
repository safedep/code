from pymongo import ASCENDING
from src.models.admin_model import Admin, AdminLogin
from src.models.staff_model import Staff, StaffLogin
from src.models.user_model import User, UserLogin
from src.establish_db_connection import database
from passlib.context import CryptContext
import random

admins = database.Admins
admins.create_index([("id", ASCENDING)], unique=True)

staffs = database.Staffs
staffs.create_index([("id", ASCENDING)], unique=True)

users = database.Users
users.create_index([("mobile", ASCENDING)], unique=True)
pwd_context = CryptContext(schemes=["bcrypt"],deprecated="auto")

def generateID():
    # 8 characters long alphanumeric id in uppercase
    id = ""
    for i in range(8):
        if random.random() < 0.5:
            id += chr(random.randint(65,90))
        else:
            id += str(random.randint(0,9))
    return id
    

def validate_admin(admin: AdminLogin):
    try:
        document = admins.find_one({"id": admin.id})

        if document == None:
            return {"ERROR":"INVALID CREDENTIALS"}
        if pwd_context.verify(admin.password, document['password']):
            del document['_id']
            del document['password']
            return {"SUCCESS": document}
        else:
            return {"ERROR":"INVALID CREDENTIALS"}
    except Exception as e:
        print(e)
        return {"ERROR":"SOME ERROR OCCURRED"}
    
def create_admin(admin: Admin):
    try:
        document = admin.dict()
        document['password'] = pwd_context.hash(admin.password)
        id = generateID()
        distincts = admins.distinct("id")

        #until we have a unique id
        while id in distincts:
            id = generateID()

        document['id'] = id
        admins.insert_one(document)
        #checking if 
        del document['password']
        del document['_id']
        return {"SUCCESS": document}
    except Exception as e:
        print(e)
        return {"ERROR":"SOME ERROR OCCURRED"}
    
def validate_staff(staff: StaffLogin):
    try:
        document = staffs.find_one({"id": staff.id})
        if document == None:
            return {"ERROR":"INVALID CREDENTIALS"}
        if pwd_context.verify(staff.password, document['password']):
            del document['_id']
            del document['password']
            return {"SUCCESS": document}
        else:
            return {"ERROR":"INVALID CREDENTIALS"}
    except Exception as e:
        print(e)
        return {"ERROR":"SOME ERROR OCCURRED"}
    
def create_staff(staff: Staff):
    try:
        document = staff.dict()
        document['password'] = pwd_context.hash(staff.password)
        id = generateID()
        distincts = staffs.distinct("id")

        #until we have a unique id
        while id in distincts:
            id = generateID()

        document['id'] = id
        
        staffs.insert_one(document)
        #checking if 
        del document['password']
        del document['_id']

        return {"SUCCESS": document}
    except Exception as e:
        print(e)
        return {"ERROR":"SOME ERROR OCCURRED"}

### NORMAL USER LOGIC

def create_user(user: User):
    try:
        document = user.dict()
        document['password'] = pwd_context.hash(user.password)
        users.insert_one(document)
        del document['password']
        del document['_id']
        del document['otp']
        return document
    except Exception as e:
        print(e)
        return {"ERROR":"SOME ERROR OCCURRED"}

def check_user(user: UserLogin, otp=None):
    try:
        document = users.find_one({"mobile":user.mobile})
        if pwd_context.verify(user.password,document['password']):
            if otp != None:
                if otp == document['otp'] and document['otp']!="EXPIRED":
                    return document
                else:
                    return {"ERROR":"INVALID OTP"}
            if document['is_verified']:
                return document
            return {"ERROR":"USER NOT VERIFIED"}
        else:
            return {"ERROR":"INVALID CREDENTIALS"}
    except Exception as e:
        print(e)
        return {"ERROR":"INVALID CREDENTIALS"}
    
def make_user_valid(mobile):
    try:
        document = users.update_one({"mobile": mobile}, {"$set": {"otp":"EXPIRED","is_verified":True}})
        if(document.matched_count>0):
            return "SUCCESS"
        else:
            return "INVALID"
    except Exception as e:
        print(e)
        return "Some Error Occurred"

def update_otp(mobile,otp):
    try:
        document = users.update_one({"mobile": mobile}, {"$set": {"otp":otp}})
        if(document.matched_count>0):
            return "SUCCESS"
        else:
            return "INVALID"
    except Exception as e:
        print(e)
        return "Some Error Occurred"
    
def update_user_token(mobile, token):
    try:
        document = users.update_one({"mobile": mobile}, {"$set": {"notification_token":token}})
        if(document.matched_count>0):
            return "SUCCESS"
        else:
            return "INVALID"
    except Exception as e:
        print(e)
        return "Some Error Occurred"
    