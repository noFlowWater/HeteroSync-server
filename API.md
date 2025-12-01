# HeteroSync Server API Specification

## Overview
- **Framework**: Gin (Go)
- **Base URL**: `http://localhost:8080`
- **Format**: JSON
- **Database**: SQLite3

## Endpoints

### Health Check
#### `GET /health`
서버 상태 확인

**Response (200)**
```json
{
  "status": "healthy"
}
```

---

### Device Management

#### `GET /api/devices`
등록된 모든 디바이스 조회

**Response (200)**
```json
[
  {
    "device_id": "psg-001",
    "device_type": "PSG",
    "last_seen": "2025-12-01T10:30:00Z"
  }
]
```

**Errors**: 500

#### `GET /api/devices/health?deviceId={deviceId}`
특정 디바이스 연결 상태 조회

**Query Parameters**
- `deviceId` (required): 디바이스 ID

**Response (200)**
```json
{
  "device_id": "psg-001",
  "device_type": "PSG",
  "is_connected": true,
  "health_status": "Healthy",
  "last_seen": "2025-12-01T10:30:00Z",
  "last_ping": "2025-12-01T10:29:55Z"
}
```

**Health Status**:
- `Healthy`: < 90초
- `Unhealthy`: 90-120초
- `Dead`: > 120초 (자동 연결 해제)

**Errors**: 404, 500

---

### Pairing Management

#### `GET /api/pairings`
모든 페어링 조회

**Response (200)**
```json
[
  {
    "id": "pairing-001",
    "psg_device_id": "psg-001",
    "pc_device_id": "pc-001",
    "created_at": "2025-12-01T10:00:00Z",
    "is_syncing": false
  }
]
```

**Errors**: 500

#### `POST /api/pairings`
새 페어링 생성

**Request Body**
```json
{
  "psg_device_id": "psg-001",
  "pc_device_id": "pc-001"
}
```

**Response (201)**
```json
{
  "id": "pairing-001",
  "psg_device_id": "psg-001",
  "pc_device_id": "pc-001",
  "created_at": "2025-12-01T10:00:00Z",
  "is_syncing": false
}
```

**Errors**: 400, 500

#### `DELETE /api/pairings/{pairingId}`
페어링 삭제

**Response (200)**
```json
{
  "message": "Pairing deleted successfully"
}
```

**Errors**: 404, 500

---

### Time Synchronization

#### `POST /api/sync/{pairingId}`
단일 동기화 수행

**Response (200)**
```json
{
  "success": true,
  "record": {
    "id": 1,
    "pairing_id": "pairing-001",
    "timestamp": "2025-12-01T10:30:00Z",
    "raw_offset_us": -1234,
    "adjusted_offset_us": -1200,
    "rtt_us": 567,
    "confidence": 0.95
  }
}
```

**Errors**: 400, 500

#### `POST /api/sync/multi`
다중 동기화 수행 (NTP 알고리즘)

**Request Body**
```json
{
  "pairing_id": "pairing-001",
  "sample_count": 10
}
```

**Response (200)**
```json
{
  "success": true,
  "result": {
    "id": 1,
    "pairing_id": "pairing-001",
    "timestamp": "2025-12-01T10:30:00Z",
    "sample_count": 10,
    "median_offset_us": -1200,
    "min_rtt_us": 450,
    "max_rtt_us": 800,
    "confidence": 0.92,
    "outliers_removed": 2
  }
}
```

**Errors**: 400, 500

#### `GET /api/sync/records`
동기화 레코드 조회 (필터링 & 페이지네이션)

**Query Parameters**
- `pairingId`: 페어링 ID 필터
- `startTime`: 시작 시간 (RFC3339)
- `endTime`: 종료 시간 (RFC3339)
- `limit`: 결과 개수 (기본값: 100)
- `offset`: 오프셋 (기본값: 0)

**Example**: `/api/sync/records?pairingId=pairing-001&limit=50&offset=0`

**Response (200)**
```json
[
  {
    "id": 1,
    "pairing_id": "pairing-001",
    "timestamp": "2025-12-01T10:30:00Z",
    "raw_offset_us": -1234,
    "adjusted_offset_us": -1200,
    "rtt_us": 567,
    "confidence": 0.95
  }
]
```

**Errors**: 400, 404, 500

#### `GET /api/sync/records/{recordId}`
특정 동기화 레코드 조회

**Response (200)**
```json
{
  "id": 1,
  "pairing_id": "pairing-001",
  "timestamp": "2025-12-01T10:30:00Z",
  "raw_offset_us": -1234,
  "adjusted_offset_us": -1200,
  "rtt_us": 567,
  "confidence": 0.95
}
```

**Errors**: 400, 404, 500

#### `GET /api/sync/aggregated`
집계된 동기화 결과 조회 (필터링 & 페이지네이션)

**Query Parameters**
- `pairingId`: 페어링 ID 필터
- `startTime`: 시작 시간 (RFC3339)
- `endTime`: 종료 시간 (RFC3339)
- `limit`: 결과 개수 (기본값: 100)
- `offset`: 오프셋 (기본값: 0)

**Response (200)**
```json
[
  {
    "id": 1,
    "pairing_id": "pairing-001",
    "timestamp": "2025-12-01T10:30:00Z",
    "sample_count": 10,
    "median_offset_us": -1200,
    "min_rtt_us": 450,
    "max_rtt_us": 800,
    "confidence": 0.92,
    "outliers_removed": 2
  }
]
```

**Errors**: 400, 404, 500

#### `GET /api/sync/aggregated/{aggregationId}`
특정 집계 결과 조회

**Response (200)**
```json
{
  "id": 1,
  "pairing_id": "pairing-001",
  "timestamp": "2025-12-01T10:30:00Z",
  "sample_count": 10,
  "median_offset_us": -1200,
  "min_rtt_us": 450,
  "max_rtt_us": 800,
  "confidence": 0.92,
  "outliers_removed": 2
}
```

**Errors**: 404, 500

---

### Auto-Sync Management

#### `POST /api/auto-sync/start`
자동 동기화 시작

**Request Body**
```json
{
  "pairing_id": "pairing-001",
  "interval_seconds": 30,
  "sample_count": 10
}
```

**Response (200)**
```json
{
  "message": "Auto-sync started",
  "pairing_id": "pairing-001",
  "interval_seconds": 30
}
```

**Errors**: 400, 404

#### `POST /api/auto-sync/stop/{pairingId}`
자동 동기화 중지

**Response (200)**
```json
{
  "message": "Auto-sync stopped",
  "pairing_id": "pairing-001"
}
```

**Errors**: 404

#### `GET /api/auto-sync/status?pairingId={pairingId}`
자동 동기화 상태 조회

**Query Parameters**
- `pairingId` (required): 페어링 ID

**Response (200)**
```json
{
  "pairing_id": "pairing-001",
  "is_running": true,
  "interval_seconds": 30,
  "last_sync": "2025-12-01T10:30:00Z"
}
```

**Errors**: 404

---

## WebSocket Protocol

### Connection
```
ws://localhost:8080/ws?deviceType={PSG|PC}&deviceId={deviceId}
```

**Query Parameters**
- `deviceType` (required): `PSG` 또는 `PC`
- `deviceId` (required): 디바이스 ID

**Upgrade Response (101)**: WebSocket 연결 성공

**Errors**: 400 (잘못된 파라미터)

### Message Types

#### CONNECTED
서버 → 클라이언트 (연결 성공)
```json
{
  "type": "CONNECTED",
  "device_id": "psg-001",
  "message": "Connected successfully"
}
```

#### PING / PONG
양방향 헬스체크
```json
{
  "type": "PING",
  "timestamp": 1733054400000
}
```
```json
{
  "type": "PONG",
  "timestamp": 1733054400000
}
```

- 애플리케이션 레벨 PING: 40초마다
- 프로토콜 레벨 PING: 54초마다

#### TIME_REQUEST
서버 → 클라이언트 (시간 동기화 요청)
```json
{
  "type": "TIME_REQUEST",
  "request_id": "req-001",
  "server_send_time": 1733054400000
}
```

#### TIME_RESPONSE
클라이언트 → 서버 (시간 동기화 응답)
```json
{
  "type": "TIME_RESPONSE",
  "request_id": "req-001",
  "server_send_time": 1733054400000,
  "client_receive_time": 1733054400100,
  "client_send_time": 1733054400200
}
```

#### ERROR
서버 → 클라이언트 (에러 메시지)
```json
{
  "type": "ERROR",
  "message": "Invalid message format"
}
```

### Connection Management
- **Read Deadline**: 60초
- **Write Deadline**: 10초
- **최대 메시지 크기**: 512 바이트
- **자동 재연결**: 클라이언트 측에서 처리

---

## Data Types

### Timestamps
- **HTTP JSON**: RFC3339 형식 (`2025-12-01T10:30:00Z`)
- **WebSocket**: Unix milliseconds (`1733054400000`)
- **내부 계산**: Microseconds (RTT, offset)

### Offset Values
- **raw_offset_us**: 네트워크 보정 전 오프셋 (마이크로초)
- **adjusted_offset_us**: 네트워크 지연 보정 후 오프셋 (마이크로초)
- **median_offset_us**: 다중 샘플의 중앙값 오프셋 (마이크로초)

### RTT (Round-Trip Time)
- 단위: 마이크로초 (microseconds)
- NTP 알고리즘에서 네트워크 지연 계산에 사용

### Confidence Score
- 범위: 0.0 ~ 1.0
- 동기화 정확도 신뢰도 점수
- 높을수록 신뢰도 높음

---

## HTTP Status Codes

| Code | Description |
|------|-------------|
| 200  | 성공 |
| 201  | 생성 성공 (POST /api/pairings) |
| 400  | 잘못된 요청 (파라미터, 검증 오류) |
| 404  | 리소스 없음 |
| 500  | 서버 내부 오류 (DB, 시스템 오류) |
| 101  | 프로토콜 전환 (WebSocket) |

---

## Error Response Format

모든 에러는 JSON 형식으로 반환:
```json
{
  "error": "Error message description"
}
```
