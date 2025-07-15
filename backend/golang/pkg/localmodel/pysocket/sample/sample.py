#!/usr/bin/env python3

import socket
import time
import threading
import logging

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class ModelServer:
    def __init__(self, host='localhost', port=8080):
        self.host = host
        self.port = port
        self.socket = None
        self.running = False
        
    def start(self):
        logger.info("Starting model server...")
        logger.info("Loading LLM model (simulated)...")
        time.sleep(5)  # Simulate 5 second model loading
        logger.info("Model loaded successfully!")
        
        self.socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self.socket.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        
        try:
            self.socket.bind((self.host, self.port))
            self.socket.listen(5)
            self.running = True
            logger.info(f"Server listening on {self.host}:{self.port}")
            
            while self.running:
                try:
                    client_socket, addr = self.socket.accept()
                    logger.info(f"Connection from {addr}")
                    
                    # Handle client in a separate thread
                    client_thread = threading.Thread(
                        target=self.handle_client,
                        args=(client_socket,)
                    )
                    client_thread.daemon = True
                    client_thread.start()
                    
                except socket.error as e:
                    if self.running:
                        logger.error(f"Socket error: {e}")
                        
        except Exception as e:
            logger.error(f"Server error: {e}")
        finally:
            self.stop()
            
    def handle_client(self, client_socket):
        try:
            while True:
                data = client_socket.recv(1024)
                if not data:
                    break
                    
                message = data.decode('utf-8').strip()
                logger.info(f"Received message: {message}")
                
                if message.startswith('infer:'):
                    # Extract the input after 'infer:'
                    input_text = message[6:]  # Remove 'infer:' prefix
                    logger.info(f"Processing inference request with input: {input_text}")
                    time.sleep(0.1)  # Sleep for 100ms
                    response = f"Processed: {input_text}\n"
                    client_socket.send(response.encode('utf-8'))
                else:
                    response = f"Unknown message format: {message}\n"
                    client_socket.send(response.encode('utf-8'))
                    
        except Exception as e:
            logger.error(f"Client handling error: {e}")
        finally:
            client_socket.close()
            logger.info("Client disconnected")
            
    def stop(self):
        logger.info("Stopping server...")
        self.running = False
        if self.socket:
            self.socket.close()

if __name__ == "__main__":
    server = ModelServer()
    try:
        server.start()
    except KeyboardInterrupt:
        logger.info("Received interrupt signal")
        server.stop()
