import cv2
from fastapi import FastAPI, WebSocket, WebSocketDisconnect
from fastapi.middleware.cors import CORSMiddleware
from typing import List
import ultralytics
import asyncio
import uvicorn
import websockets.exceptions

FAST_API_PORT = 5005
app = FastAPI()
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

ultralytics.checks()
model = ultralytics.YOLO('yolov8x.pt')
print(f"YOLO model loaded !")

# Store connected WebSocket clients
connected_clients = []


class WebSocketConnection:
    def __init__(self, websocket: WebSocket):
        self.websocket = websocket


# async def send_people_count(websocket: WebSocket):
#     await websocket.accept()
#     connection = WebSocketConnection(websocket)
#     connected_clients.append(connection)
#     print("Connected")
#     try:
#         results = model("rtmp://127.0.0.1/mystream", stream=True, save=False)
#         for result in results:
#             people_in_frame = list(result.boxes.cls).count(0)
#             print("People in frame", people_in_frame)
#             await websocket.send_text(str(people_in_frame))
#             print("Sent to frontend !")

#     except websockets.exceptions.ConnectionClosedError:
#         connected_clients.remove(connection)

# async def websocket_endpoint(websocket: WebSocket):
#     print("Here")
#     await send_people_count(websocket)




@app.websocket("/ws")
async def process_stream(websocket: WebSocket):
    await websocket.accept()
    await asyncio.sleep(0.1)

    # Start the video stream
    cap = cv2.VideoCapture('rtmp://127.0.0.1/mystream')

    try:
      while True:
          # Capture a frame from the video stream
          ret, frame = cap.read()

          if not ret:
              break

          # Perform object detection on the frame
          predictions = model(frame)

          people_in_frame = list(predictions[0].boxes.cls).count(0)
          # people_in_frame = sum(p['class'] == 'person' for p in predictions)
          print("People in frame", people_in_frame)
          
          # Send the predictions back to the client
          # await websocket.send_text(str(people_in_frame))

          # Send the number of people in the frame back to the client
          await websocket.send_text(str(people_in_frame))

          # Sleep briefly to allow other asyncio tasks to run
          await asyncio.sleep(0.1)
    except (websockets.exceptions.ConnectionClosedError, websockets.exceptions.ConnectionClosedOK):
        print("Client disconnected")
        cap.release()

    # Close the video stream
    cap.release()



if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=FAST_API_PORT)
