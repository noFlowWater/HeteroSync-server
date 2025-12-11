# Database Schema

HeteroSync Server는 SQLite3 데이터베이스를 사용하여 페어링, 동기화 기록, 집계 결과를 영구 저장합니다.

## 의존성

- `github.com/mattn/go-sqlite3 v1.14.22` - SQLite 드라이버

---

## 데이터베이스 파일

**기본 경로**: `./time-sync.db`
**환경변수**: `DB_PATH`

```bash
# 커스텀 경로로 실행
DB_PATH=/data/heterosync.db ./server
```

---

## 테이블 구조

### 1. `time_sync_records` (개별 측정)

단일 시간 동기화 측정값을 저장합니다.

```sql
CREATE TABLE time_sync_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device1_id TEXT NOT NULL,
    device1_type TEXT NOT NULL,
    device1_t1 INTEGER,
    device1_t2 INTEGER,
    device1_t3 INTEGER,
    device1_t4 INTEGER,
    device1_offset INTEGER,
    device1_delay INTEGER,
    device1_rtt INTEGER,
    device2_id TEXT NOT NULL,
    device2_type TEXT NOT NULL,
    device2_t1 INTEGER,
    device2_t2 INTEGER,
    device2_t3 INTEGER,
    device2_t4 INTEGER,
    device2_offset INTEGER,
    device2_delay INTEGER,
    device2_rtt INTEGER,
    time_difference INTEGER,
    status TEXT NOT NULL,
    error_message TEXT,
    created_at INTEGER NOT NULL
);

CREATE INDEX idx_time_sync_device1 ON time_sync_records(device1_id);
CREATE INDEX idx_time_sync_device2 ON time_sync_records(device2_id);
CREATE INDEX idx_time_sync_created ON time_sync_records(created_at);
```

#### 컬럼 설명

| 컬럼 | 타입 | NULL | 설명 |
|------|------|------|------|
| `id` | INTEGER | NO | Primary Key (auto-increment) |
| `device1_id` | TEXT | NO | Device 1 ID |
| `device1_type` | TEXT | NO | Device 1 타입 (PSG, WATCH, MOBILE) |
| `device1_t1` | INTEGER | YES | Device1 NTP T1 타임스탬프 (ms) |
| `device1_t2` | INTEGER | YES | Device1 NTP T2 타임스탬프 (ms) |
| `device1_t3` | INTEGER | YES | Device1 NTP T3 타임스탬프 (ms) |
| `device1_t4` | INTEGER | YES | Device1 NTP T4 타임스탬프 (ms) |
| `device1_offset` | INTEGER | YES | Device1 오프셋 (ms) |
| `device1_delay` | INTEGER | YES | Device1 지연 (ms) |
| `device1_rtt` | INTEGER | YES | Device1 RTT (μs) |
| `device2_id` | TEXT | NO | Device 2 ID |
| `device2_type` | TEXT | NO | Device 2 타입 (PSG, WATCH, MOBILE) |
| `device2_t1` | INTEGER | YES | Device2 NTP T1 타임스탬프 (ms) |
| `device2_t2` | INTEGER | YES | Device2 NTP T2 타임스탬프 (ms) |
| `device2_t3` | INTEGER | YES | Device2 NTP T3 타임스탬프 (ms) |
| `device2_t4` | INTEGER | YES | Device2 NTP T4 타임스탬프 (ms) |
| `device2_offset` | INTEGER | YES | Device2 오프셋 (ms) |
| `device2_delay` | INTEGER | YES | Device2 지연 (ms) |
| `device2_rtt` | INTEGER | YES | Device2 RTT (μs) |
| `time_difference` | INTEGER | YES | **원본** 시간 오프셋 (ms), 네트워크 보정 **없음** |
| `status` | TEXT | NO | SUCCESS, PARTIAL, FAILED |
| `error_message` | TEXT | YES | 에러 메시지 (FAILED인 경우) |
| `created_at` | INTEGER | NO | 생성 시간 (Unix milliseconds) |

**중요**: `time_difference`는 원본(raw) 오프셋입니다. 네트워크 지연 보정은 NTPSelector가 다중 샘플링 시 적용합니다. 단일 측정 API를 사용할 경우 클라이언트가 RTT를 고려하여 직접 보정해야 합니다.

#### 인덱스
- `idx_time_sync_device1`: device1_id로 빠른 조회
- `idx_time_sync_device2`: device2_id로 빠른 조회
- `idx_time_sync_created`: created_at으로 시간 범위 조회

---

### 2. `aggregated_sync_results` (NTP 집계 결과)

다중 샘플링 결과를 저장합니다. 모든 오프셋은 **네트워크 지연 보정이 적용된** 값입니다.

```sql
CREATE TABLE aggregated_sync_results (
    aggregation_id TEXT PRIMARY KEY,
    pairing_id TEXT NOT NULL,
    best_offset INTEGER NOT NULL,
    median_offset INTEGER NOT NULL,
    mean_offset REAL NOT NULL,
    offset_std_dev REAL NOT NULL,
    min_rtt INTEGER NOT NULL,
    max_rtt INTEGER NOT NULL,
    mean_rtt REAL NOT NULL,
    confidence REAL NOT NULL,
    jitter REAL NOT NULL,
    total_samples INTEGER NOT NULL,
    valid_samples INTEGER NOT NULL,
    outlier_count INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE INDEX idx_aggregated_pairing ON aggregated_sync_results(pairing_id);
CREATE INDEX idx_aggregated_created ON aggregated_sync_results(created_at);
```

#### 컬럼 설명

| 컬럼 | 타입 | NULL | 설명 |
|------|------|------|------|
| `aggregation_id` | TEXT | NO | Primary Key (UUID) |
| `pairing_id` | TEXT | NO | 페어링 ID |
| `best_offset` | INTEGER | NO | **최적 오프셋** (ms), 네트워크 보정 **적용됨** |
| `median_offset` | INTEGER | NO | 중앙값 오프셋 (ms), 네트워크 보정 적용됨 |
| `mean_offset` | REAL | NO | 평균 오프셋 (ms), 네트워크 보정 적용됨 |
| `offset_std_dev` | REAL | NO | 오프셋 표준편차 (ms) |
| `min_rtt` | INTEGER | NO | 최소 RTT (μs) |
| `max_rtt` | INTEGER | NO | 최대 RTT (μs) |
| `mean_rtt` | REAL | NO | 평균 RTT (μs) |
| `confidence` | REAL | NO | 신뢰도 점수 (0.0~1.0) |
| `jitter` | REAL | NO | 네트워크 변동성 (μs) |
| `total_samples` | INTEGER | NO | 총 샘플 수 |
| `valid_samples` | INTEGER | NO | 유효 샘플 수 |
| `outlier_count` | INTEGER | NO | 제거된 이상값 개수 |
| `created_at` | INTEGER | NO | 생성 시간 (Unix milliseconds) |

**권장**: EDF 후처리에는 `best_offset` 값을 사용하세요. 이 값은 NTP 알고리즘이 선택한 가장 신뢰할 수 있는 오프셋입니다.

#### 인덱스
- `idx_aggregated_pairing`: pairing_id로 페어링별 집계 조회
- `idx_aggregated_created`: created_at으로 시간 범위 조회

---

### 3. `aggregation_measurements` (연결 테이블)

집계 결과와 개별 측정을 연결합니다.

```sql
CREATE TABLE aggregation_measurements (
    aggregation_id TEXT NOT NULL,
    measurement_id INTEGER NOT NULL,
    PRIMARY KEY (aggregation_id, measurement_id),
    FOREIGN KEY (aggregation_id) REFERENCES aggregated_sync_results(aggregation_id),
    FOREIGN KEY (measurement_id) REFERENCES time_sync_records(id)
);

CREATE INDEX idx_agg_measurement_agg ON aggregation_measurements(aggregation_id);
CREATE INDEX idx_agg_measurement_meas ON aggregation_measurements(measurement_id);
```

#### 컬럼 설명

| 컬럼 | 타입 | NULL | 설명 |
|------|------|------|------|
| `aggregation_id` | TEXT | NO | 집계 결과 ID |
| `measurement_id` | INTEGER | NO | 개별 측정 record ID |

**사용 목적**: 특정 집계 결과를 만드는데 사용된 모든 개별 측정을 추적할 수 있습니다.

#### 인덱스
- `idx_agg_measurement_agg`: aggregation_id로 해당 집계의 모든 측정 조회
- `idx_agg_measurement_meas`: measurement_id로 역방향 조회

---

### 4. `pairings` (페어링 영구 저장)

디바이스 페어링 정보와 Auto-Sync 설정을 영구 저장합니다.

```sql
CREATE TABLE pairings (
    pairing_id TEXT PRIMARY KEY,
    device1_id TEXT NOT NULL,
    device2_id TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    auto_sync_interval_sec INTEGER,
    auto_sync_sample_count INTEGER,
    auto_sync_interval_ms INTEGER,
    UNIQUE(device1_id, device2_id)
);

CREATE INDEX idx_pairing_device1 ON pairings(device1_id);
CREATE INDEX idx_pairing_device2 ON pairings(device2_id);
CREATE UNIQUE INDEX idx_pairing_devices ON pairings(device1_id, device2_id);
```

#### 컬럼 설명

| 컬럼 | 타입 | NULL | 설명 |
|------|------|------|------|
| `pairing_id` | TEXT | NO | Primary Key (UUID) |
| `device1_id` | TEXT | NO | Device 1 ID |
| `device2_id` | TEXT | NO | Device 2 ID |
| `created_at` | INTEGER | NO | 생성 시간 (Unix milliseconds) |
| `auto_sync_interval_sec` | INTEGER | YES | Auto-Sync 주기 (초, NULL 가능) |
| `auto_sync_sample_count` | INTEGER | YES | Auto-Sync 샘플 수 (NULL 가능) |
| `auto_sync_interval_ms` | INTEGER | YES | Auto-Sync 샘플 간격 (ms, NULL 가능) |

**특징**:
- 페어링 정보가 **영구 저장**되어 서버 재시작 후에도 유지
- 디바이스 재연결 시 **자동 복구**에 사용됨
- Auto-Sync 설정도 함께 저장되어 복구 시 동일 설정으로 재시작
- `UNIQUE(device1_id, device2_id)` 제약조건으로 중복 페어링 방지

#### 인덱스
- `idx_pairing_device1`: device1_id로 페어링 조회
- `idx_pairing_device2`: device2_id로 페어링 조회
- `idx_pairing_devices`: (device1_id, device2_id) UNIQUE 인덱스 (중복 방지)

---

## 데이터 타입

### Timestamps
- **HTTP JSON**: RFC3339 형식 (`2025-12-01T10:30:00Z`)
- **WebSocket**: Unix milliseconds (`1733054400000`)
- **데이터베이스**: Unix milliseconds (INTEGER)
- **NTP 타임스탬프**: Milliseconds (T1-T4)

### Offset Values
- **단위**: milliseconds (ms)
- **deviceOffset**: NTP 계산된 클록 오프셋
- **timeDifference**: 두 디바이스 간 RAW 시간 차이 (보정 전)

### RTT (Round-Trip Time)
- **단위**: microseconds (μs)
- NTP 알고리즘에서 네트워크 지연 계산에 사용
- Device1RTT, Device2RTT는 각 디바이스와 서버 간의 왕복 시간

### Confidence Score
- **범위**: 0.0 ~ 1.0 (REAL)
- 동기화 정확도 신뢰도 점수
- 높을수록 신뢰도 높음

---

## 데이터 조회 예시

### 1. 특정 디바이스의 모든 동기화 기록 조회

```sql
SELECT * FROM time_sync_records
WHERE device1_id = 'psg-001' OR device2_id = 'psg-001'
ORDER BY created_at DESC
LIMIT 100;
```

### 2. 특정 페어링의 집계 결과 조회

```sql
SELECT * FROM aggregated_sync_results
WHERE pairing_id = 'pairing-uuid-123'
ORDER BY created_at DESC;
```

### 3. 집계 결과와 개별 측정 JOIN 조회

```sql
SELECT
    asr.aggregation_id,
    asr.best_offset,
    asr.confidence,
    tsr.id AS measurement_id,
    tsr.time_difference,
    tsr.device1_rtt,
    tsr.device2_rtt
FROM aggregated_sync_results asr
JOIN aggregation_measurements am ON asr.aggregation_id = am.aggregation_id
JOIN time_sync_records tsr ON am.measurement_id = tsr.id
WHERE asr.aggregation_id = 'agg-uuid-xxx';
```

### 4. 시간 범위로 동기화 기록 조회

```sql
SELECT * FROM time_sync_records
WHERE created_at BETWEEN 1733054400000 AND 1733140800000
ORDER BY created_at ASC;
```

### 5. 디바이스의 모든 페어링 조회 (자동 복구용)

```sql
SELECT * FROM pairings
WHERE device1_id = 'psg-001' OR device2_id = 'psg-001';
```

---

## 데이터 유지보수

### 데이터베이스 백업

```bash
# SQLite 데이터베이스 백업
sqlite3 time-sync.db ".backup time-sync-backup.db"

# 또는 파일 복사
cp time-sync.db time-sync-$(date +%Y%m%d).db
```

### 오래된 데이터 정리

```sql
-- 30일 이전 개별 측정 기록 삭제
DELETE FROM time_sync_records
WHERE created_at < (strftime('%s', 'now') - 30*24*3600) * 1000;

-- 90일 이전 집계 결과 삭제
DELETE FROM aggregated_sync_results
WHERE created_at < (strftime('%s', 'now') - 90*24*3600) * 1000;

-- 데이터베이스 크기 최적화
VACUUM;
```

### 데이터베이스 통계

```sql
-- 테이블별 레코드 수
SELECT 'time_sync_records' AS table_name, COUNT(*) AS count FROM time_sync_records
UNION ALL
SELECT 'aggregated_sync_results', COUNT(*) FROM aggregated_sync_results
UNION ALL
SELECT 'pairings', COUNT(*) FROM pairings;

-- 데이터베이스 파일 크기 확인
SELECT page_count * page_size AS size_bytes
FROM pragma_page_count(), pragma_page_size();
```

---

## 마이그레이션

스키마 변경이 필요한 경우 마이그레이션 스크립트를 사용하세요:

```sql
-- 예시: 새 컬럼 추가
ALTER TABLE pairings ADD COLUMN description TEXT;

-- 인덱스 추가
CREATE INDEX idx_pairing_created ON pairings(created_at);
```

---

## 성능 최적화

### 인덱스 사용 확인

```sql
-- 쿼리 실행 계획 확인
EXPLAIN QUERY PLAN
SELECT * FROM time_sync_records
WHERE device1_id = 'psg-001'
ORDER BY created_at DESC
LIMIT 100;
```

### 자주 사용하는 쿼리 최적화

```sql
-- 복합 인덱스 생성 (device + time)
CREATE INDEX idx_time_sync_device1_created
ON time_sync_records(device1_id, created_at DESC);

CREATE INDEX idx_time_sync_device2_created
ON time_sync_records(device2_id, created_at DESC);
```

---

## 참고 사항

- SQLite는 단일 라이터 잠금을 사용하므로, 높은 쓰기 부하에서는 Write-Ahead Logging (WAL) 모드 권장
- 페어링 삭제 시 관련 동기화 기록은 자동으로 삭제되지 않음 (의도적 설계)
- 집계 결과 삭제 시 `aggregation_measurements` 테이블의 연결도 함께 삭제 필요
