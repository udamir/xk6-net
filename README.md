# xk6-net

A [k6 extension](https://grafana.com/docs/k6/latest/extensions/) for TCP socket operations with message handling capabilities

## Build

To build a `k6` binary with this plugin, first ensure you have the prerequisites:

- [Go toolchain](https://go101.org/article/go-toolchain.html)
- Git

Then:

1. Install `xk6`:

```sh
go install go.k6.io/xk6/cmd/xk6@latest
```

2. Build the binary:

```sh
xk6 build --with github.com/udamir/xk6-net@latest
```

## Example

```javascript
import net from 'k6/x/net';
import { check } from 'k6';

export default function () {
  const socket = new net.Socket();

  // Connect with configuration
  socket.connect('localhost', 5000, {
    // Connection timeout in milliseconds
    timeout: 5000,

    // Length field length for length-prefixed framing (for binary encoding)
    // If 0, message length determined by maxLength or delimiter
    // If both lengthFieldLength and maxLength are 0 with binary encoding, no "message" events
    lengthFieldLength: 4,  // 0 | 1 | 2 | 4 | 8

    // Maximum message length in bytes (0 = unlimited)
    maxLength: 1 * 1024 * 1024,

    // Message encoding: "binary" or "utf-8"
    encoding: "binary",

    // Delimiter for UTF-8 messages (removed from decoded message)
    delimiter: "",

    // Enable TLS/SSL
    tls: false,

    // Server name for SNI and certificate verification
    serverName: "",

    // Skip TLS certificate verification (NOT recommended for production)
    insecureSkipVerify: false,
  })

  socket.on("data", (data) => {
    // handle raw data (Uint8Array)
    console.log("Received raw data:", data.length, "bytes");
  })

  socket.on("message", (message) => {
    // handle unpacked message (decoded based on encoding)
    console.log("Received message:", message);
  })

  socket.on("error", (error) => {
    // handle errors
    console.error("Socket error:", error);
  })

  socket.on("end", () => {
    // handle socket termination
    console.log("Socket connection ended");
  })

  // send message with length header (4 bytes big-endian)
  const messageData = new Uint8Array([1, 2, 3, 4, 5]);
  socket.send(messageData);

  // send raw data without length header
  socket.write(messageData);

  // close socket
  socket.close();
}
```

## TypeScript Support

xk6-net includes comprehensive TypeScript type definitions for enhanced development experience.

### Installation

1. Install k6 types for your project:

```bash
npm install --save-dev @types/k6
```

2. Download the type definitions from GitHub:

```bash
# Create a types directory if it doesn't exist
mkdir -p types

# Download the k6-x-net.d.ts file from GitHub
curl -o types/k6-x-net.d.ts https://raw.githubusercontent.com/udamir/xk6-net/master/types/k6-x-net.d.ts
```

3. Ensure your `tsconfig.json` includes the types directory:

```json
{
  "compilerOptions": {
    "typeRoots": ["./node_modules/@types", "./types"]
  }
}
```

### Type Definitions

The module exports the following TypeScript interfaces:

- `SocketConfig` - Configuration options for Socket.connect()
- `Socket` - Main socket class with all methods
- `DataEventHandler` - Type for "data" event handlers
- `MessageEventHandler` - Type for "message" event handlers  
- `ErrorEventHandler` - Type for "error" event handlers
- `EndEventHandler` - Type for "end" event handlers

### Usage with TypeScript

```typescript
import net from 'k6/x/net';
import type { SocketConfig } from 'k6/x/net';

export default function() {
  const socket = new net.Socket();

  // TypeScript will provide autocomplete and type checking for SocketConfig
  const config: SocketConfig = {
    timeout: 5000,
    lengthFieldLength: 4,       // Type: 0 | 1 | 2 | 4 | 8
    maxLength: 1024 * 1024,
    encoding: 'binary',         // Type: 'utf-8' | 'binary'
    delimiter: '',
    tls: false,
    serverName: '',
    insecureSkipVerify: false
  };

  socket.connect('localhost', 5000, config);

  // Type-safe event handlers
  socket.on('data', (data: Uint8Array) => {
    console.log(`Received ${data.length} bytes`);
  });

  socket.on('error', (error: Error) => {
    console.error('Socket error:', error.message);
  });

  // Send with automatic length header
  const message = new Uint8Array([1, 2, 3, 4, 5]);
  socket.send(message);

  socket.close();
}
```

# License

MIT
