// TypeScript definitions for xk6-net
// Project: https://github.com/udamir/xk6-net
// Definitions by: Damir Yusipov <https://github.com/udamir>

declare module 'k6/x/net' {
  /**
   * Socket configuration options
   */
  export interface SocketConfig {
    /**
     * Length of the length field in bytes.
     * If lengthFieldLength is 0, then the length field is not used, and the message length is determined by maxLength.
     * If encoding is "binary" and lengthFieldLength and maxLength are both 0, then message event will not be emitted.
     * @default 0
     */
    lengthFieldLength?: number;

    /**
     * Maximum message length in bytes.
     * If maxLength is 0, then the maximum message length is not limited.
     * @default 0
     */
    maxLength?: number;

    /**
     * Encoding of the message: "utf-8" or "binary".
     * If encoding is "utf-8", then the message is decoded using utf-8 encoding.
     * If encoding is "binary", then the message is decoded using binary encoding.
     * @default "binary"
     */
    encoding?: "utf-8" | "binary";

    /**
     * Delimiter for the message.
     * If encoding is "utf-8" and delimiter is not empty, then it will be used as delimiter for splitting messages.
     * Note that delimiter will be removed from the decoded message.
     * @default ""
     */
    delimiter?: string;
  }

  /**
   * Event handler function for raw data events
   */
  export type DataEventHandler = (data: Uint8Array) => void;

  /**
   * Event handler function for decoded message events
   */
  export type MessageEventHandler = (message: Uint8Array | string) => void;

  /**
   * Event handler function for error events
   */
  export type ErrorEventHandler = (error: Error) => void;

  /**
   * Event handler function for connection end events
   */
  export type EndEventHandler = () => void;

  /**
   * Socket event names
   */
  export type SocketEventType = "data" | "message" | "error" | "end";

  /**
   * Socket event handler map
   */
  export interface SocketEventHandlers {
    data: DataEventHandler;
    message: MessageEventHandler;
    error: ErrorEventHandler;
    end: EndEventHandler;
  }

  /**
   * TCP Socket with message handling capabilities for k6 load testing
   */
  export class Socket {
    /**
     * Create a new Socket instance
     * @param config Socket configuration options
     */
    constructor(config?: SocketConfig);

    /**
     * Establish a TCP connection to the specified address
     * @param addr Address in format "host:port" (e.g., "localhost:8080")
     * @param timeoutMs Connection timeout in milliseconds
     * @returns Promise that resolves when connection is established
     * @throws Error if connection fails
     */
    connect(addr: string, timeoutMs?: number): void;

    /**
     * Register an event handler for the specified event type
     * @param event Event type to listen for
     * @param handler Event handler function
     */
    on<T extends SocketEventType>(
      event: T,
      handler: SocketEventHandlers[T]
    ): void;

    /**
     * Send a message with length header (if lengthFieldLength > 0)
     * @param data Message data to send
     * @returns Promise that resolves when message is sent
     * @throws Error if socket is not connected or send fails
     */
    send(data: Uint8Array): void;

    /**
     * Send raw data without any headers
     * @param data Raw data to send
     * @returns Promise that resolves when data is sent
     * @throws Error if socket is not connected or write fails
     */
    write(data: Uint8Array): void;

    /**
     * Close the socket connection
     * @returns Promise that resolves when connection is closed
     */
    close(): void;
  }

  /**
   * Net module for creating Socket instances
   */
  export interface NetModule {
    /**
     * Create a new Socket instance with the given configuration
     * @param config Socket configuration options
     * @returns New Socket instance
     */
    Socket: new (config?: SocketConfig) => Socket;
  }

  /**
   * Default export of the net module
   */
  const net: NetModule;
  export default net;
}

// Global augmentation for k6 modules
declare global {
  namespace K6 {
    namespace Modules {
      interface Modules {
        'k6/x/net': typeof import('k6/x/net');
      }
    }
  }
}

export {};
