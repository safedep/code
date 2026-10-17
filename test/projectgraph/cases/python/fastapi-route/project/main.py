from fastapi import APIRouter, FastAPI
import yaml

app = FastAPI()
router = APIRouter()


@router.get("/items")
async def items():
    return yaml.safe_load("a: 1")


@app.on_event("startup")
def startup():
    pass
