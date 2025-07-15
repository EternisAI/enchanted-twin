#!/usr/bin/env python3

import socket
import time

def main():
    print("Starting model server...")
    print("Loading LLM model (simulated)...")
    time.sleep(5)  # Simulate 5 second model loading
    print("Model loaded successfully!")
    
    server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    server.bind(('localhost', 8080))
    server.listen(5)
    print("Server listening on localhost:8080")
    
    try:
        while True:
            client, addr = server.accept()
            print(f"Connection from {addr}")
            
            data = client.recv(1024).decode('utf-8').strip()
            print(f"Received: {data}")
            
            if data.startswith('infer:'):
                input_text = data[6:]  # Remove 'infer:' prefix
                print(f"Processing inference: {input_text}")
                time.sleep(0.1)  # Sleep for 100ms
                response = f"Processed: {input_text}\n"
                client.send(response.encode('utf-8'))
            else:
                response = f"Unknown message: {data}\n"
                client.send(response.encode('utf-8'))
                
            client.close()
            print("Client disconnected")
            
    except KeyboardInterrupt:
        print("Shutting down...")
    finally:
        server.close()

if __name__ == "__main__":
    main()
