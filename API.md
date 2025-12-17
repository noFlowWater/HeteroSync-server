# HeteroSync Server API Specification

## Overview
- **Framework**: Gin (Go)
- **Base URL**: `http://localhost:8080`
- **Format**: JSON
- **Database**: SQLite3

---

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

## Device Management

### `GET /api/devices`
등록된 모든 디바이스 조회

**Response (200)**
```json
[
  {
    "deviceId": "psg-001",
    "deviceType": "PSG",
    "connectedAt": "2025-12-11T10:30:00Z"
  },
  {
    "deviceId": "watch-001",
    "deviceType": "WATCH",
    "connectedAt": "2025-12-11T10:31:00Z"
  }
]
```

**Device Types**: `PSG`, `WATCH`, `MOBILE`

**Errors**: 500

---

### `GET /api/devices/health`
디바이스 연결 상태 조회

**Query Parameters**
- `deviceId` (optional): 특정 디바이스 ID로 필터링

**Examples**
```bash
# 모든 디바이스 건강도 조회
GET /api/devices/health

# 특정 디바이스 건강도 조회
GET /api/devices/health?deviceId=psg-001
```

**Response (200) - All Devices**
```json
[
  {
    "deviceId": "psg-001",
    "deviceType": "PSG",
    "connectedAt": "2025-12-11T10:30:00Z",
    "lastPingSent": "2025-12-11T10:35:20Z",
    "lastPongRecv": "2025-12-11T10:35:20Z",
    "lastRtt": 15,
    "isHealthy": true,
    "timeSinceLastPong": 5000
  }
]
```

**Response (200) - Single Device**
```json
{
  "deviceId": "psg-001",
  "deviceType": "PSG",
  "connectedAt": "2025-12-11T10:30:00Z",
  "lastPingSent": "2025-12-11T10:35:20Z",
  "lastPongRecv": "2025-12-11T10:35:20Z",
  "lastRtt": 15,
  "isHealthy": true,
  "timeSinceLastPong": 5000
}
```

**Health Status**:
- `Healthy`: < 90초
- `Unhealthy`: 90-120초
- `Dead`: > 120초 (자동 연결 해제)

**Errors**: 404 (디바이스 없음), 500

---

## Pairing Management

### `GET /api/pairings`
모든 페어링 조회

**Response (200)**
```json
[
  {
    "pairingId": "550e8400-e29b-41d4-a716-446655440000",
    "device1Id": "psg-001",
    "device2Id": "watch-001",
    "createdAt": "2025-12-11T10:00:00Z",
    "autoSyncIntervalSec": 600,
    "autoSyncSampleCount": 15,
    "autoSyncIntervalMs": 200
  }
]
```

**Note**: DB에 저장된 모든 페어링을 조회합니다 (in-memory가 아닌 영구 저장소).

**Errors**: 500

---

### `POST /api/pairings`
새 페어링 생성

**Request Body**
```json
{
  "device1Id": "psg-001",
  "device2Id": "watch-001",
  "autoSyncIntervalSec": 600,
  "autoSyncSampleCount": 15,
  "autoSyncIntervalMs": 200
}
```

**Request Parameters**
| 필드 | 타입 | 필수 | 설명 |
|------|------|------|------|
| `device1Id` | string | ✅ | Device 1 ID |
| `device2Id` | string | ✅ | Device 2 ID |
| `autoSyncIntervalSec` | int | ❌ | Auto-Sync 주기(초), 기본값: 환경변수 또는 600 |
| `autoSyncSampleCount` | int | ❌ | 샘플링 횟수, 기본값: 환경변수 또는 15 |
| `autoSyncIntervalMs` | int | ❌ | 샘플 간격(ms), 기본값: 환경변수 또는 200 |

**Response (200)**
```json
{
  "pairingId": "550e8400-e29b-41d4-a716-446655440000"
}
```

**자동 동작**:
- 페어링 정보를 DB에 영구 저장
- Auto-Sync가 자동으로 시작 (백그라운드)

**Errors**: 400 (잘못된 요청), 500

---

### `DELETE /api/pairings/{pairingId}`
페어링 삭제

**Example**
```bash
DELETE /api/pairings/550e8400-e29b-41d4-a716-446655440000
```

**Response (200)**
```json
{
  "message": "Pairing deleted successfully"
}
```

**자동 동작**:
- Auto-Sync 중지
- in-memory에서 삭제
- DB에서 영구 삭제

**Errors**: 404 (페어링 없음), 500

---

## Time Synchronization

### `POST /api/sync/{pairingId}`
단일 동기화 수행

**Example**
```bash
POST /api/sync/550e8400-e29b-41d4-a716-446655440000
```

**Response (200)**
```json
{
  "success": true,
  "record": {
    "id": 123,
    "device1Id": "psg-001",
    "device1Type": "PSG",
    "device1T1": 1727870400000,
    "device1T2": 1727870400100,
    "device1T3": 1727870400102,
    "device1T4": 1727870400105,
    "device1Offset": 50,
    "device1Delay": 5,
    "device1Rtt": 5000,
    "device2Id": "watch-001",
    "device2Type": "WATCH",
    "device2T1": 1727870400010,
    "device2T2": 1727870400130,
    "device2T3": 1727870400132,
    "device2T4": 1727870400145,
    "device2Offset": 75,
    "device2Delay": 13,
    "device2Rtt": 8000,
    "timeDifference": -333,
    "status": "SUCCESS",
    "createdAt": 1727870401000
  }
}
```

**Response Fields**
- `device1T1`: Server → Device1 요청 송신 (ms)
- `device1T2`: Device1 요청 수신 (ms)
- `device1T3`: Device1 응답 송신 (ms)
- `device1T4`: Server ← Device1 응답 수신 (ms)
- `device1Offset`: NTP 오프셋 (ms)
- `device1Delay`: NTP 지연 (ms)
- `device1Rtt`: Round-Trip Time (μs)
- `timeDifference`: **원본 오프셋** (ms, 네트워크 보정 **없음**)

**⚠️ 중요**: 단일 측정의 `timeDifference`는 네트워크 보정이 적용되지 않은 원본 값입니다. 정밀한 동기화를 위해서는 **NTP 다중 샘플링**(`/api/sync/multi`)을 사용하세요.

**Errors**: 400, 404, 500

---

### `POST /api/sync/multi`
NTP 다중 샘플링 동기화 (권장)

**Request Body**
```json
{
  "pairing_id": "550e8400-e29b-41d4-a716-446655440000",
  "sample_count": 10,
  "interval_ms": 200,
  "timeout_sec": 5
}
```

**Request Parameters**
| 필드 | 타입 | 필수 | 설명 |
|------|------|------|------|
| `pairing_id` | string | ✅ | 페어링 ID |
| `sample_count` | int | ❌ | 측정 횟수, 기본값: 8, 최대: 20 |
| `interval_ms` | int | ❌ | 측정 간격(ms), 기본값: 200 |
| `timeout_sec` | int | ❌ | 각 측정 타임아웃(초), 기본값: 5 |

**Response (200)**
```json
{
  "success": true,
  "result": {
    "aggregation_id": "agg-uuid-xxx",
    "pairing_id": "550e8400-e29b-41d4-a716-446655440000",
    "best_offset": -150,
    "median_offset": -150,
    "mean_offset": -151.2,
    "offset_std_dev": 3.5,
    "min_rtt": 5000,
    "max_rtt": 15000,
    "mean_rtt": 8500.0,
    "jitter": 2000.0,
    "total_samples": 10,
    "valid_samples": 8,
    "outlier_count": 2,
    "created_at": 1727870401000
  }
}
```

**Response Fields**
- `best_offset`: **최적 오프셋** (ms, 네트워크 보정 **적용됨**)
- `jitter`: 네트워크 변동성 (μs, 낮을수록 안정적)
- `offset_std_dev`: 오프셋 표준편차 (ms)

**Errors**: 400, 404, 500

---

### `GET /api/sync/records`
동기화 레코드 조회

**Query Parameters**
- `pairingId` (optional): 페어링 ID 필터
- `deviceId` (optional): 디바이스 ID 필터
- `startTime` (optional): 시작 시간 (RFC3339)
- `endTime` (optional): 종료 시간 (RFC3339)
- `limit` (optional): 결과 개수 (기본값: 50, 최대: 1000)
- `offset` (optional): 페이지네이션 오프셋 (기본값: 0)

**Examples**
```bash
# 전체 조회
GET /api/sync/records?limit=50&offset=0

# 특정 디바이스 조회
GET /api/sync/records?deviceId=psg-001&limit=50

# 시간 범위로 조회
GET /api/sync/records?startTime=2025-12-01T00:00:00Z&endTime=2025-12-02T23:59:59Z
```

**Response (200)**
```json
[
  {
    "id": 123,
    "device1Id": "psg-001",
    "device1Type": "PSG",
    "device1Rtt": 5000,
    "device2Id": "watch-001",
    "device2Type": "WATCH",
    "device2Rtt": 8000,
    "timeDifference": -333,
    "status": "SUCCESS",
    "createdAt": 1727870401000
  }
]
```

**Errors**: 400, 500

---

### `GET /api/sync/records/{recordId}`
특정 동기화 레코드 조회

**Example**
```bash
GET /api/sync/records/123
```

**Response (200)**
```json
{
  "id": 123,
  "device1Id": "psg-001",
  "device1Type": "PSG",
  "device1T1": 1727870400000,
  "device1T2": 1727870400100,
  "device1T3": 1727870400102,
  "device1T4": 1727870400105,
  "device1Offset": 50,
  "device1Rtt": 5000,
  "device2Id": "watch-001",
  "device2Type": "WATCH",
  "device2T1": 1727870400010,
  "device2T2": 1727870400130,
  "device2T3": 1727870400132,
  "device2T4": 1727870400145,
  "device2Offset": 75,
  "device2Rtt": 8000,
  "timeDifference": -333,
  "status": "SUCCESS",
  "createdAt": 1727870401000
}
```

**Errors**: 404, 500

---

### `GET /api/sync/aggregated`
집계된 동기화 결과 조회

**Query Parameters**
- `pairingId` (optional): 페어링 ID 필터
- `startTime` (optional): 시작 시간 (RFC3339)
- `endTime` (optional): 종료 시간 (RFC3339)
- `limit` (optional): 결과 개수 (기본값: 50, 최대: 1000)
- `offset` (optional): 페이지네이션 오프셋 (기본값: 0)

**Examples**
```bash
# 전체 조회
GET /api/sync/aggregated?limit=50&offset=0

# 특정 페어링 조회
GET /api/sync/aggregated?pairingId=550e8400-e29b-41d4-a716-446655440000

# 시간 범위로 조회
GET /api/sync/aggregated?startTime=2025-12-01T00:00:00Z&endTime=2025-12-02T23:59:59Z
```

**Response (200)**
```json
[
  {
    "aggregation_id": "agg-uuid-1",
    "pairing_id": "550e8400-e29b-41d4-a716-446655440000",
    "best_offset": -150,
    "median_offset": -150,
    "mean_offset": -151.2,
    "created_at": 1727870401000
  }
]
```

**Errors**: 400, 500

---

### `GET /api/sync/aggregated/{aggregationId}`
특정 집계 결과 조회 (모든 개별 측정 포함)

**Example**
```bash
GET /api/sync/aggregated/agg-uuid-xxx
```

**Response (200)**
```json
{
  "aggregation_id": "agg-uuid-xxx",
  "pairing_id": "550e8400-e29b-41d4-a716-446655440000",
  "best_offset": -150,
  "median_offset": -150,
  "mean_offset": -151.2,
  "offset_std_dev": 3.5,
  "min_rtt": 5000,
  "max_rtt": 15000,
  "mean_rtt": 8500.0,
  "confidence": 0.94,
  "jitter": 2000.0,
  "total_samples": 10,
  "valid_samples": 8,
  "outlier_count": 2,
  "measurements": [
    {
      "id": 123,
      "device1Id": "psg-001",
      "timeDifference": -150,
      "status": "SUCCESS"
    }
  ],
  "created_at": 1727870401000
}
```

**Errors**: 404, 500

---

## Auto-Sync Management

### `POST /api/auto-sync/start`
자동 동기화 시작

**Request Body**
```json
{
  "pairing_id": "550e8400-e29b-41d4-a716-446655440000",
  "interval_sec": 600,
  "sample_count": 15,
  "interval_ms": 200
}
```

**Request Parameters**
| 필드 | 타입 | 필수 | 설명 |
|------|------|------|------|
| `pairing_id` | string | ✅ | 페어링 ID |
| `interval_sec` | int | ❌ | 동기화 주기(초), 기본값: 600 |
| `sample_count` | int | ❌ | NTP 샘플 수, 기본값: 15 |
| `interval_ms` | int | ❌ | 샘플 간격(ms), 기본값: 200 |

**Response (200)**
```json
{
  "message": "auto-sync started",
  "pairing_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Errors**:
- 400: 잘못된 요청
- 404: 페어링 없음
- 409: 이미 실행 중

---

### `POST /api/auto-sync/stop/{pairingId}`
자동 동기화 중지

**Example**
```bash
POST /api/auto-sync/stop/550e8400-e29b-41d4-a716-446655440000
```

**Response (200)**
```json
{
  "message": "auto-sync stopped",
  "pairing_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Errors**:
- 404: Auto-Sync이 실행 중이 아님

---

### `GET /api/auto-sync/status`
자동 동기화 상태 조회

**Query Parameters**
- `pairingId` (optional): 특정 페어링 ID로 필터링

**Examples**
```bash
# 모든 Auto-Sync 작업 조회
GET /api/auto-sync/status

# 특정 페어링의 Auto-Sync 상태 조회
GET /api/auto-sync/status?pairingId=550e8400-e29b-41d4-a716-446655440000
```

**Response (200) - All Jobs**
```json
{
  "jobs": [
    {
      "pairing_id": "550e8400-e29b-41d4-a716-446655440000",
      "status": "RUNNING",
      "config": {
        "pairing_id": "550e8400-e29b-41d4-a716-446655440000",
        "interval_sec": 600,
        "sample_count": 15,
        "interval_ms": 200
      },
      "started_at": "2025-12-11T10:00:00Z",
      "last_sync_at": "2025-12-11T10:05:00Z",
      "last_sync_success": true,
      "last_error": "",
      "total_syncs": 5,
      "failed_syncs": 0
    }
  ]
}
```

**Response (200) - Single Job**
```json
{
  "pairing_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "RUNNING",
  "config": {
    "interval_sec": 600,
    "sample_count": 15
  },
  "started_at": "2025-12-11T10:00:00Z",
  "last_sync_at": "2025-12-11T10:05:00Z",
  "last_sync_success": true,
  "total_syncs": 5,
  "failed_syncs": 0
}
```

**Status Values**: `RUNNING`, `STOPPED`, `FAILED`

**Errors**: 404

---

## WebSocket Protocol

### Connection

**Endpoint**
```
ws://localhost:8080/ws?deviceType={PSG|WATCH|MOBILE}&deviceId={deviceId}
```

**Query Parameters**
- `deviceType` (required): `PSG`, `WATCH`, 또는 `MOBILE`
- `deviceId` (required): 디바이스 ID

**Example**
```javascript
const ws = new WebSocket('ws://localhost:8080/ws?deviceType=PSG&deviceId=psg-001');
```

---

### Message Types

#### CONNECTED
서버 → 클라이언트 (연결 성공)

```json
{
  "type": "CONNECTED",
  "deviceId": "psg-001",
  "serverTime": 1733054400000
}
```

---

#### PING / PONG
양방향 헬스체크

**서버 → 클라이언트**
```json
{
  "type": "PING",
  "timestamp": 1733054400000
}
```

**클라이언트 → 서버 (필수)**
```json
{
  "type": "PONG",
  "timestamp": 1733054400000
}
```

- 애플리케이션 레벨 PING: 20초마다
- 프로토콜 레벨 PING: 54초마다
- PONG 미응답 120초 시 자동 연결 해제

---

#### TIME_REQUEST
서버 → 클라이언트 (시간 동기화 요청)

```json
{
  "type": "TIME_REQUEST",
  "requestId": "req-uuid-xxx",
  "pairingId": "pairing-uuid-xxx"
}
```

---

#### TIME_RESPONSE
클라이언트 → 서버 (시간 동기화 응답)

```json
{
  "type": "TIME_RESPONSE",
  "requestId": "req-uuid-xxx",
  "receiveTime": 1733054400100,
  "sendTime": 1733054400200
}
```

**필드 설명**:
- `receiveTime` (T2): 클라이언트가 TIME_REQUEST를 받은 시각 (ms)
- `sendTime` (T3): 클라이언트가 TIME_RESPONSE를 보낸 시각 (ms)

**NTP 타임스탬프**:
```
T1: Server → Device 요청 전송 (서버 기록)
T2: Device 요청 수신 (receiveTime)
T3: Device 응답 전송 (sendTime)
T4: Server ← Device 응답 수신 (서버 기록)

Offset = ((T2-T1) + (T3-T4)) / 2
RTT = (T4-T1) - (T3-T2)
```

---

#### ERROR
서버 → 클라이언트 (에러 메시지)

```json
{
  "type": "ERROR",
  "code": "INVALID_MESSAGE",
  "message": "Invalid message format"
}
```

---

## Data Types

### Timestamps
- **HTTP JSON**: RFC3339 형식 (`2025-12-11T10:30:00Z`)
- **WebSocket**: Unix milliseconds (`1733054400000`)
- **Database**: Unix milliseconds (INTEGER)
- **NTP 타임스탬프**: Milliseconds (T1-T4)

### Offset Values
- **단위**: milliseconds (ms)
- **timeDifference**: 두 디바이스 간 RAW 시간 차이 (보정 전)
- **best_offset**: NTP 알고리즘으로 계산된 최적 오프셋 (보정 후)

### RTT (Round-Trip Time)
- **단위**: microseconds (μs)
- NTP 알고리즘에서 네트워크 지연 계산에 사용

---

## HTTP Status Codes

| Code | Description |
|------|-------------|
| 200  | 성공 |
| 201  | 생성 성공 |
| 400  | 잘못된 요청 (파라미터, 검증 오류) |
| 404  | 리소스 없음 |
| 409  | 충돌 (예: Auto-Sync 이미 실행 중) |
| 500  | 서버 내부 오류 |
| 101  | 프로토콜 전환 (WebSocket) |

---

## Error Response Format

모든 에러는 JSON 형식으로 반환:

```json
{
  "error": "Error message description"
}
```

**Examples**:
```json
// 404 Not Found
{
  "error": "pairing not found: 550e8400-e29b-41d4-a716-446655440000"
}

// 400 Bad Request
{
  "error": "device1Id is required"
}

// 409 Conflict
{
  "error": "auto-sync already running for pairing: 550e8400-e29b-41d4-a716-446655440000"
}
```

---

## 더 보기

- [README.md](README.md) - 프로젝트 개요 및 빠른 시작
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) - 아키텍처 및 NTP 알고리즘 상세
- [docs/WEBSOCKET.md](docs/WEBSOCKET.md) - WebSocket 프로토콜 상세 및 클라이언트 구현 가이드
- [docs/DATABASE.md](docs/DATABASE.md) - 데이터베이스 스키마
- [docs/GUIDE.md](docs/GUIDE.md) - 사용 가이드 및 시나리오
