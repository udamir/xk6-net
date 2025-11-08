// TypeScript example demonstrating xk6-net usage with proper type safety
import net from 'k6/x/net';
import { check } from 'k6';

// Example configuration demonstrating type safety
export default function () {
  // Create socket instance
  const socket = new net.Socket();

  console.log("Connecting to server...");

  // Connect with typed configuration
  try {
    socket.connect('localhost', 5000, {
      timeout: 5000,               // Connection timeout in milliseconds
      lengthFieldLength: 4,         // Length field length: 0 | 1 | 2 | 4 | 8
      maxLength: 1024 * 1024,       // Maximum message length in bytes (0 = unlimited)
      encoding: "binary",           // Encoding: "utf-8" | "binary"
      delimiter: "",                // Delimiter for UTF-8 messages
      tls: false,                   // Enable TLS/SSL
      serverName: "",               // Server name for TLS verification
      insecureSkipVerify: false     // Skip TLS certificate verification
    });
  } catch (error) {
    console.error("Connection failed:", error);
    return;
  }

  let messagesReceived = 0;
  let errorsReceived = 0;

  // Type-safe event handlers
  socket.on("data", (data: Uint8Array) => {
    console.log(`Received raw data: ${data.length} bytes`);
    messagesReceived++;
  });

  socket.on("message", (message: Uint8Array | string) => {
    if (message instanceof Uint8Array) {
      console.log(`Received binary message: ${message.length} bytes`);
    } else {
      console.log(`Received text message: ${message}`);
    }
  });

  socket.on("error", (error: Error) => {
    console.error("Socket error:", error.message);
    errorsReceived++;
  });

  socket.on("end", () => {
    console.log("Connection ended");
  });

  // Type-safe message sending
  const sendMessage = (payload: Uint8Array): void => {
    try {
      // send() automatically adds length header based on lengthFieldLength config
      socket.send(payload);
      console.log(`Sent message: ${payload.length} bytes`);
    } catch (error) {
      console.error("Send failed:", error);
    }
  };

  const sendRawData = (data: Uint8Array): void => {
    try {
      // write() sends raw data without length header
      socket.write(data);
      console.log(`Sent raw data: ${data.length} bytes`);
    } catch (error) {
      console.error("Write failed:", error);
    }
  };

  // Example protobuf-style message (cserver compatible)
  const heartbeatMessage = new Uint8Array([
    0x00, 0x00, 0x00, 0x33,  // Payload type: HEARTBEAT (51) 
    0x08, 0x33                // Simple protobuf message
  ]);

  const authMessage = new Uint8Array([
    0x00, 0x00, 0x00, 0x05,  // Payload type: AUTH_REQ (5)
    0x08, 0x01, 0x12, 0x04, 0x74, 0x65, 0x73, 0x74  // Auth payload
  ]);

  // Send messages
  sendMessage(authMessage);
  sendMessage(heartbeatMessage);

  // Send raw data example
  const rawData = new Uint8Array([0x01, 0x02, 0x03, 0x04]);
  sendRawData(rawData);

  // Properly close connection
  socket.close();

  // Type-safe checks
  check(null, {
    'messages received': () => messagesReceived > 0,
    'no errors occurred': () => errorsReceived === 0,
  });
}

// k6 test options with proper typing
export const options = {
  vus: 1,
  duration: '10s',
  thresholds: {
    checks: ['rate>0.9'],
  },
};

// Advanced example: Custom message builder with types
interface MessageBuilder {
  payloadType: number;
  payload: Uint8Array;
}

class ProtobufMessageBuilder implements MessageBuilder {
  constructor(
    public payloadType: number,
    public payload: Uint8Array
  ) {}

  build(): Uint8Array {
    const header = new ArrayBuffer(4);
    const headerView = new DataView(header);
    headerView.setUint32(0, this.payloadType, false); // Big endian

    const result = new Uint8Array(4 + this.payload.length);
    result.set(new Uint8Array(header), 0);
    result.set(this.payload, 4);

    return result;
  }
}

// Usage with type safety
export function advancedExample() {
  const socket = new net.Socket();

  socket.connect('localhost', 6000, {
    timeout: 3000,
    lengthFieldLength: 4,
    encoding: "binary",
    maxLength: 10 * 1024 * 1024 // 10MB
  });

  // Type-safe message building
  const messageBuilder = new ProtobufMessageBuilder(
    51, // Heartbeat type
    new Uint8Array([0x08, 0x33])
  );

  const message = messageBuilder.build();
  socket.send(message);
  
  socket.close();
}
