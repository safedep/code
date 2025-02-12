from pymongo import MongoClient
from src.config import DB_URL, USERNAME, PASSWORD
#FOR MONGO ATLAS

client = MongoClient(f"mongodb+srv://{USERNAME}:{PASSWORD}@{DB_URL}/?retryWrites=true&w=majority")

#local connection
# client =  MongoClient("mongodb://localhost:27017/")

database = client.RailRakshak
