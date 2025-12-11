# HeteroSync Server

WebSocket 기반 이종 디바이스 간 NTP 정밀 시간 동기화 서버

## 개요

HeteroSync Server는 웨어러블 디바이스(갤럭시 워치)와 PSG 장비 간의 정밀한 시간 동기화를 위한 서버입니다. NTP(Network Time Protocol) 알고리즘을 사용하여 네트워크 지연을 보정하고, 다중 샘플링으로 신뢰도 높은 시간 오프셋을 계산합니다.

**주요 사용 사례**: 수면 측정 시 PSG 장비의 EDF 파일과 웨어러블 디바이스 데이터의 타임스탬프를 후처리에서 정밀하게 정렬

## 주요 기능

- **WebSocket 연결 관리**: PSG, Watch, Mobile 디바이스가 WebSocket으로 서버에 연결
- **PING/PONG 연결 모니터링**: 이중 PING 시스템으로 연결 상태 실시간 추적 (애플리케이션 + 프로토콜 레벨)
- **NTP 다중 샘플링**: NTP 알고리즘 기반 정밀 시간 동기화 (RTT 필터링, 이상값 제거, 네트워크 보정)
- **자동 페어링 복구**: 페어링 정보를 DB에 영구 저장하여 디바이스 재연결 시 자동 복구
- **자동 주기적 동기화**: 페어링 생성 시 Auto-Sync가 자동 시작 (백그라운드 실행)
- **시간 동기화 이력**: 모든 측정 기록 및 NTP 집계 결과를 SQLite에 영구 저장

## 아키텍처

```
┌─────────────┐         WebSocket         ┌──────────────┐
│  PSG 장비    │◄─────────────────────────►│              │
└─────────────┘                           │              │
                                          │  HeteroSync  │
┌─────────────┐         WebSocket         │    Server    │
│ Galaxy Watch│◄─────────────────────────►│              │
└─────────────┘                           │              │
                                          └──────────────┘
                                                  │
                                                  ▼
                                            ┌──────────┐
                                            │ SQLite DB│
                                            └──────────┘
```

**지원 디바이스 타입**: `PSG`, `WATCH`, `MOBILE`

## 빠른 시작

### 요구사항

- Go 1.21.0 이상

### 설치 및 실행

```bash
# 빌드
go build -o server ./cmd/server

# 기본 설정으로 실행 (포트 8080, DB: ./time-sync.db)
./server

# 환경변수로 설정 변경
PORT=9000 DB_PATH=/path/to/database.db ./server

# Auto-Sync 기본값 설정
AUTO_SYNC_INTERVAL_SEC=120 AUTO_SYNC_SAMPLE_COUNT=10 AUTO_SYNC_INTERVAL_MS=300 ./server
```

### 개발 모드 실행

```bash
go run ./cmd/server/main.go
```

## 기본 사용 예시

### 1. 디바이스 연결 확인

```bash
# 연결된 디바이스 조회
curl http://localhost:8080/api/devices | jq

# 응답:
[
  {"deviceId": "psg-001", "deviceType": "PSG", "connectedAt": "2025-12-11T10:30:00Z"},
  {"deviceId": "watch-001", "deviceType": "WATCH", "connectedAt": "2025-12-11T10:31:00Z"}
]
```

### 2. 페어링 생성

```bash
# 두 디바이스를 페어링 (Auto-Sync 자동 시작)
curl -X POST http://localhost:8080/api/pairings \
  -H "Content-Type: application/json" \
  -d '{
    "device1Id": "psg-001",
    "device2Id": "watch-001"
  }'

# 응답:
{"pairingId": "550e8400-e29b-41d4-a716-446655440000"}
```

**자동 동작**:
- 페어링 정보가 DB에 영구 저장
- Auto-Sync가 자동으로 시작 (기본: 600초 주기, 15샘플)
- 백그라운드에서 주기적으로 NTP 다중 샘플링 수행

### 3. NTP 다중 샘플링 동기화 (권장)

```bash
# 10회 측정 후 최적 오프셋 계산
curl -X POST http://localhost:8080/api/sync/multi \
  -H "Content-Type: application/json" \
  -d '{
    "pairing_id": "550e8400-e29b-41d4-a716-446655440000",
    "sample_count": 10,
    "interval_ms": 200
  }'

# 응답:
{
  "success": true,
  "result": {
    "best_offset": -150,
    "confidence": 0.94,
    "jitter": 2000.0,
    "valid_samples": 8
  }
}
```

**해석**:
- `best_offset: -150ms` → PSG 클럭이 Watch보다 150ms 느림
- `confidence: 0.94` → 매우 신뢰할 수 있는 측정 (0.0~1.0)
- `jitter: 2000μs` → 네트워크 안정적 (2ms 변동)

### 4. 동기화 이력 조회

```bash
# NTP 집계 결과 조회
curl "http://localhost:8080/api/sync/aggregated?pairingId=550e8400-e29b-41d4-a716-446655440000&limit=10" | jq

# Auto-Sync 상태 조회
curl "http://localhost:8080/api/auto-sync/status?pairingId=550e8400-e29b-41d4-a716-446655440000" | jq
```

## WebSocket 연결

### 클라이언트 연결 예시

```javascript
// PSG 장비 연결
const ws1 = new WebSocket('ws://localhost:8080/ws?deviceType=PSG&deviceId=psg-001');

// Galaxy Watch 연결
const ws2 = new WebSocket('ws://localhost:8080/ws?deviceType=WATCH&deviceId=watch-001');
```

### PING/PONG 처리 (필수)

```javascript
ws.onmessage = (event) => {
  const message = JSON.parse(event.data);

  if (message.type === 'PING') {
    // ⚠️ 필수: PING 수신 시 즉시 PONG 응답
    ws.send(JSON.stringify({
      type: 'PONG',
      timestamp: Date.now()
    }));
  } else if (message.type === 'TIME_REQUEST') {
    // 시간 동기화 요청 처리
    ws.send(JSON.stringify({
      type: 'TIME_RESPONSE',
      requestId: message.requestId,
      receiveTime: Date.now(),  // T2
      sendTime: Date.now()       // T3
    }));
  }
};
```

## 환경 변수

| 변수 | 설명 | 기본값 |
|------|------|--------|
| `PORT` | 서버 포트 | `8080` |
| `DB_PATH` | SQLite DB 파일 경로 | `./time-sync.db` |
| `AUTO_SYNC_INTERVAL_SEC` | Auto-Sync 기본 주기 (초) | `600` |
| `AUTO_SYNC_SAMPLE_COUNT` | Auto-Sync 기본 샘플 수 | `15` |
| `AUTO_SYNC_INTERVAL_MS` | Auto-Sync 샘플 간격 (ms) | `200` |

**사용 예시**:
```bash
# Auto-Sync 기본값을 커스터마이즈하여 서버 시작
AUTO_SYNC_INTERVAL_SEC=120 \
AUTO_SYNC_SAMPLE_COUNT=10 \
AUTO_SYNC_INTERVAL_MS=300 \
./server
```

## 의존성

- `github.com/gin-gonic/gin v1.10.0` - HTTP 웹 프레임워크
- `github.com/gorilla/websocket v1.5.3` - WebSocket 라이브러리
- `github.com/mattn/go-sqlite3 v1.14.22` - SQLite 드라이버
- `github.com/google/uuid v1.6.0` - UUID 생성

## 문서

더 자세한 정보는 아래 문서를 참고하세요:

- **[API.md](API.md)** - REST API 및 WebSocket 명세
  - 모든 API 엔드포인트 상세
  - 요청/응답 예시
  - 에러 코드 및 처리

- **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)** - 아키텍처 및 NTP 알고리즘
  - 프로젝트 구조 상세
  - NTP 다중 샘플링 알고리즘 설명
  - 네트워크 지연 보정 원리
  - 동작 흐름

- **[docs/WEBSOCKET.md](docs/WEBSOCKET.md)** - WebSocket 프로토콜
  - 메시지 타입 및 프로토콜
  - PING/PONG 메커니즘
  - 클라이언트 구현 가이드
  - 연결 유지 모범 사례

- **[docs/DATABASE.md](docs/DATABASE.md)** - 데이터베이스 스키마
  - 테이블 구조 및 컬럼 설명
  - 인덱스 정보
  - 쿼리 예시
  - 데이터 유지보수

- **[docs/GUIDE.md](docs/GUIDE.md)** - 사용 가이드
  - NTP 다중 샘플링 활용
  - Auto-Sync 설정 및 모니터링
  - 페어링 자동 복구 시나리오
  - EDF 후처리 예시
  - 트러블슈팅

## 주요 시나리오

### 1. EDF 후처리를 위한 시간 동기화

```bash
# 측정 시작 전
curl -X POST http://localhost:8080/api/sync/multi \
  -d '{"pairing_id":"pair-123","sample_count":10}'
# → best_offset: -150ms

# 측정 종료 후 (8시간 뒤)
curl -X POST http://localhost:8080/api/sync/multi \
  -d '{"pairing_id":"pair-123","sample_count":10}'
# → best_offset: -182ms

# 클럭 드리프트: -32ms / 8시간 = -1.1 ppm
# 후처리에서 선형 보간으로 정밀 정렬
```

### 2. 장기간 연속 측정 (Auto-Sync)

```bash
# 페어링 생성 시 Auto-Sync 설정 (1시간마다)
curl -X POST http://localhost:8080/api/pairings \
  -d '{
    "device1Id":"psg-001",
    "device2Id":"watch-001",
    "autoSyncIntervalSec":3600,
    "autoSyncSampleCount":15
  }'

# 이후 자동으로 1시간마다 NTP 다중 샘플링 수행
# 결과는 자동으로 DB에 저장되어 후처리에 활용 가능
```

### 3. 디바이스 재연결 자동 복구

```bash
# 페어링 생성 → DB에 저장 & Auto-Sync 시작
curl -X POST http://localhost:8080/api/pairings \
  -d '{"device1Id":"psg-001","device2Id":"watch-001"}'

# 네트워크 끊김 → in-memory 삭제, Auto-Sync 중단
# (DB의 페어링은 유지)

# 디바이스 재연결 → 자동으로 페어링 및 Auto-Sync 복구
# 수동 개입 없이 연속 데이터 수집 재개
```

## 라이센스

MIT
