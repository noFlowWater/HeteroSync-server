# WebSocket Protocol

## Connection

### Endpoint
```
ws://localhost:8080/ws?deviceType={PSG|WATCH|MOBILE}&deviceId={deviceId}
```

### Query Parameters
- `deviceType` (required): 디바이스 타입
  - `PSG`: Polysomnography 장비
  - `WATCH`: 웨어러블 워치
  - `MOBILE`: 모바일 디바이스
- `deviceId` (required): 고유 디바이스 ID

### Connection Examples
```javascript
// PSG 장비 연결
const ws1 = new WebSocket('ws://localhost:8080/ws?deviceType=PSG&deviceId=psg-001');

// Galaxy Watch 연결
const ws2 = new WebSocket('ws://localhost:8080/ws?deviceType=WATCH&deviceId=watch-001');
```

---

## Message Protocol

### Message Types

#### 1. CONNECTED
서버 → 클라이언트 (연결 성공)

```json
{
  "type": "CONNECTED",
  "deviceId": "psg-001",
  "serverTime": 1727870400000
}
```

**필드 설명**:
- `type`: 메시지 타입 (항상 "CONNECTED")
- `deviceId`: 연결된 디바이스 ID
- `serverTime`: 서버의 현재 시간 (Unix milliseconds)

---

#### 2. TIME_REQUEST
서버 → 클라이언트 (시간 동기화 요청)

```json
{
  "type": "TIME_REQUEST",
  "requestId": "req-uuid-xxx",
  "pairingId": "pairing-uuid-xxx"
}
```

**필드 설명**:
- `type`: 메시지 타입 (항상 "TIME_REQUEST")
- `requestId`: 요청 ID (응답 매칭에 사용)
- `pairingId`: 페어링 ID

**클라이언트 동작**:
1. TIME_REQUEST 수신 시각 기록 (T2)
2. TIME_RESPONSE 전송 시각 기록 (T3)
3. TIME_RESPONSE 메시지 전송

---

#### 3. TIME_RESPONSE
클라이언트 → 서버 (시간 동기화 응답)

```json
{
  "type": "TIME_RESPONSE",
  "requestId": "req-uuid-xxx",
  "receiveTime": 1727870400123,
  "sendTime": 1727870400125
}
```

**필드 설명**:
- `type`: 메시지 타입 (항상 "TIME_RESPONSE")
- `requestId`: 요청 ID (TIME_REQUEST의 requestId와 동일)
- `receiveTime`: 클라이언트가 TIME_REQUEST를 받은 시각 (T2, milliseconds)
- `sendTime`: 클라이언트가 TIME_RESPONSE를 보낸 시각 (T3, milliseconds)

**NTP 타임스탬프**:
```
T1: Server → Device 요청 전송 시각 (서버가 기록)
T2: Device가 요청 수신 시각 (receiveTime)
T3: Device가 응답 전송 시각 (sendTime)
T4: Server ← Device 응답 수신 시각 (서버가 기록)

Offset = ((T2-T1) + (T3-T4)) / 2
RTT = (T4-T1) - (T3-T2)
```

---

#### 4. PING
서버 → 클라이언트 (연결 유지)

```json
{
  "type": "PING",
  "timestamp": 1727870400000
}
```

**필드 설명**:
- `type`: 메시지 타입 (항상 "PING")
- `timestamp`: PING 전송 시각 (Unix milliseconds)

**클라이언트 동작**:
- PING 수신 즉시 PONG 메시지 전송 필요 (필수)

---

#### 5. PONG
클라이언트 → 서버 (연결 확인)

```json
{
  "type": "PONG",
  "timestamp": 1727870400015
}
```

**필드 설명**:
- `type`: 메시지 타입 (항상 "PONG")
- `timestamp`: PONG 전송 시각 (Unix milliseconds)

---

#### 6. ERROR
서버 → 클라이언트 (에러 메시지)

```json
{
  "type": "ERROR",
  "code": "INVALID_MESSAGE",
  "message": "Invalid message format"
}
```

**필드 설명**:
- `type`: 메시지 타입 (항상 "ERROR")
- `code`: 에러 코드
- `message`: 에러 메시지

---

## PING/PONG 연결 모니터링 프로토콜

서버는 **이중 PING 시스템**을 사용하여 WebSocket 연결 상태를 지속적으로 모니터링합니다.

### 1️⃣ WebSocket 프로토콜 레벨 PING (네트워크 계층)

WebSocket 표준 프레임을 사용한 낮은 레벨의 연결 유지:

| 속성 | 값 |
|------|-----|
| **전송 주기** | 54초마다 |
| **프레임 타입** | WebSocket Ping Frame (opcode 0x9) |
| **처리 방식** | 브라우저/라이브러리가 자동으로 Pong 응답 |
| **타임아웃** | 60초 (Pong 미수신 시 연결 종료) |
| **목적** | 네트워크 계층 연결 유지, NAT/방화벽 세션 타임아웃 방지 |

**클라이언트 측 처리**:
- 대부분의 WebSocket 라이브러리에서 자동 처리
- 별도 구현 불필요 (브라우저/OS 레벨에서 자동 응답)

---

### 2️⃣ 애플리케이션 레벨 PING/PONG (JSON 메시지)

애플리케이션 계층에서 명시적으로 연결 상태를 확인하고 RTT를 측정:

| 속성 | 값 |
|------|-----|
| **전송 주기** | 20초마다 |
| **메시지 형식** | JSON (`{"type": "PING", "timestamp": ...}`) |
| **처리 방식** | 클라이언트가 명시적으로 PONG 응답 필요 |
| **RTT 측정** | PING 전송 ~ PONG 수신 시간 차이 |
| **목적** | 연결 건강도 확인, RTT 측정, 애플리케이션 응답성 검증 |

**타임라인 예시**:
```
T=0s    : WebSocket 연결 수립
T=20s   : 서버 → PING (App-level)
T=20.015s: 클라이언트 → PONG (RTT: 15ms)
T=40s   : 서버 → PING
T=54s   : 서버 → Ping (Protocol-level)
T=54.002s: 클라이언트 → Pong (자동)
T=60s   : 서버 → PING (App-level)
...
```

---

### 연결 상태 판정 기준

| 상태 | 조건 | 설명 |
|------|------|------|
| 🟢 **Healthy** | Last PONG < 90초 전 | 정상 연결, `isHealthy: true` |
| 🟡 **Unhealthy** | Last PONG > 90초 전 | 응답 지연, `isHealthy: false` |
| 🔴 **Dead** | Last PONG > 120초 전 | 자동 연결 해제 (30초마다 체크) |

**상태 전이**:
```
[연결 수립] → [Healthy]
              ↓ (90초 PONG 없음)
           [Unhealthy]
              ↓ (120초 PONG 없음)
           [Dead] → [연결 해제]
```

---

## 클라이언트 구현 가이드

### 필수 구현: PING에 대한 PONG 응답

```javascript
const websocket = new WebSocket('ws://localhost:8080/ws?deviceId=psg-001&deviceType=PSG');

websocket.onmessage = (event) => {
  const message = JSON.parse(event.data);

  switch (message.type) {
    case 'PING':
      // ⚠️ 필수: PING 수신 시 즉시 PONG 응답
      websocket.send(JSON.stringify({
        type: 'PONG',
        timestamp: Date.now()
      }));
      console.log('Sent PONG response');
      break;

    case 'CONNECTED':
      console.log('Connected to server:', message);
      break;

    case 'TIME_REQUEST':
      // 시간 동기화 요청 처리
      handleTimeRequest(message);
      break;
  }
};

websocket.onerror = (error) => {
  console.error('WebSocket error:', error);
};

websocket.onclose = (event) => {
  console.log('WebSocket closed:', event.code, event.reason);
  // 120초 타임아웃으로 닫힌 경우: code 1006 (Abnormal Closure)
};
```

---

### TIME_REQUEST 처리 구현

```javascript
function handleTimeRequest(message) {
  const receiveTime = Date.now(); // T2: 요청 받은 시각

  // 시간 응답 전송
  const sendTime = Date.now(); // T3: 응답 보내는 시각

  websocket.send(JSON.stringify({
    type: 'TIME_RESPONSE',
    requestId: message.requestId,
    receiveTime: receiveTime,
    sendTime: sendTime
  }));

  console.log('Sent TIME_RESPONSE:', {
    requestId: message.requestId,
    T2: receiveTime,
    T3: sendTime
  });
}
```

---

### 권장 구현: 연결 건강도 모니터링

```javascript
class HealthMonitor {
  constructor(websocket) {
    this.ws = websocket;
    this.lastPingReceived = Date.now();
    this.lastPongSent = Date.now();

    // 1분마다 건강도 체크
    setInterval(() => this.checkHealth(), 60000);
  }

  onPingReceived() {
    this.lastPingReceived = Date.now();

    // PONG 즉시 전송
    this.ws.send(JSON.stringify({
      type: 'PONG',
      timestamp: Date.now()
    }));
    this.lastPongSent = Date.now();
  }

  checkHealth() {
    const timeSinceLastPing = Date.now() - this.lastPingReceived;

    if (timeSinceLastPing > 90000) {
      console.warn('⚠️ Connection unhealthy: No PING for', timeSinceLastPing, 'ms');
      // 재연결 로직 실행 가능
    } else {
      console.log('✅ Connection healthy');
    }
  }

  async queryServerHealth() {
    // REST API로 서버 측 건강도 확인
    const response = await fetch('http://localhost:8080/api/devices/health?deviceId=psg-001');
    const health = await response.json();
    console.log('Server-side health:', health);
    return health;
  }
}

// 사용 예시
const monitor = new HealthMonitor(websocket);

websocket.onmessage = (event) => {
  const message = JSON.parse(event.data);

  if (message.type === 'PING') {
    monitor.onPingReceived();
  } else if (message.type === 'TIME_REQUEST') {
    handleTimeRequest(message);
  }
};
```

---

## 연결 유지 모범 사례

### ✅ DO (권장)

- PING 수신 즉시 PONG 응답 (지연 최소화)
- 주기적으로 REST API로 연결 건강도 확인 (`/api/devices/health`)
- 120초 타임아웃 전에 재연결 로직 준비
- `onclose` 이벤트에서 자동 재연결 구현

### ❌ DON'T (비권장)

- PONG 응답 지연 (블로킹 작업 중 PING 무시)
- 프로토콜 레벨 PING만 의존 (애플리케이션 레벨 무시)
- 타임아웃 후 무한 재연결 시도 (백오프 전략 사용)

---

## 디버깅 및 모니터링

### 서버 로그 확인

```bash
# PING/PONG 관련 로그
tail -f server.log | grep -E "PING|PONG|Dead connection"

# 출력 예시:
# 2025/10/18 14:35:20 Received PONG from client psg-001, RTT: 15ms
# 2025/10/18 14:37:45 Dead connection detected: watch-001 (no PONG for 125s)
```

### REST API로 실시간 모니터링

```bash
# 실시간 건강도 모니터링 스크립트
watch -n 5 'curl -s http://localhost:8080/api/devices/health | jq'

# 또는 특정 디바이스만
watch -n 5 'curl -s "http://localhost:8080/api/devices/health?deviceId=psg-001" | jq'
```

### 연결 해제 원인 분석

```javascript
websocket.onclose = (event) => {
  switch (event.code) {
    case 1000:
      console.log('Normal closure');
      break;
    case 1006:
      console.log('Abnormal closure - possibly 120s timeout');
      break;
    default:
      console.log('Connection closed:', event.code, event.reason);
  }
};
```

---

## Connection Settings

### Timeouts
- **Read Deadline**: 60초
- **Write Deadline**: 10초
- **최대 메시지 크기**: 512 바이트

### Reconnection Strategy

```javascript
class ReconnectingWebSocket {
  constructor(url) {
    this.url = url;
    this.reconnectDelay = 1000; // 초기 1초
    this.maxReconnectDelay = 30000; // 최대 30초
    this.reconnectAttempts = 0;
    this.connect();
  }

  connect() {
    this.ws = new WebSocket(this.url);

    this.ws.onopen = () => {
      console.log('WebSocket connected');
      this.reconnectDelay = 1000; // 재연결 성공 시 리셋
      this.reconnectAttempts = 0;
    };

    this.ws.onclose = () => {
      console.log('WebSocket closed, reconnecting...');
      this.scheduleReconnect();
    };

    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
  }

  scheduleReconnect() {
    this.reconnectAttempts++;
    const delay = Math.min(
      this.reconnectDelay * Math.pow(2, this.reconnectAttempts),
      this.maxReconnectDelay
    );

    console.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);
    setTimeout(() => this.connect(), delay);
  }
}

// 사용 예시
const rws = new ReconnectingWebSocket('ws://localhost:8080/ws?deviceId=psg-001&deviceType=PSG');
```
