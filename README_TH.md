# ALO Backend API (ภาษาไทย)

Backend API สำหรับจัดการ Kubernetes โดยมีระบบ Authentication, RBAC, ตรวจสอบ Manifest, คิวงาน (Queue Worker), ขั้นตอน Build/Deploy, ดู Logs และตรวจสอบสถานะระบบ

## ความสามารถด้านความปลอดภัย

- ยืนยันตัวตนผ่าน `X-API-Key` หรือ JWT Bearer Token
- RBAC กำหนดสิทธิ์ตามผู้ใช้งาน (principal)
- จำกัดสิทธิ์ตาม namespace ของแต่ละผู้ใช้
- ตรวจสอบนโยบาย Manifest ก่อน deploy:
  - บล็อก privileged container
  - บล็อก hostPath volume
  - บังคับเพดาน cpu/memory ตาม policy ของ project
- Rate limiting ต่อผู้ใช้งานที่ยืนยันตัวตนแล้ว

## โครงสร้างข้อมูล (SQLite)

- `projects`: เก็บ repo, environment, namespace, resource limits, service account
- `services`: เก็บ image และจำนวน replicas ต่อ project

## วิธีรัน

```bash
go run ./src
```

หรือเลือกวิธีตั้ง environment variables ตามที่อธิบายไว้ข้างล่าง:

## Environment variables

- `PORT` (ค่าเริ่มต้น `8080`)
- `DB_PATH` (ค่าเริ่มต้น `./alo.db`)
- `API_KEY_ADMIN` (ค่าเริ่มต้น `local-admin-key`)
- `API_KEY_DEPLOYER` (ค่าเริ่มต้น `local-deployer-key`)
- `JWT_SECRET` (ไม่บังคับ, ถ้าตั้งค่าจะเปิดใช้ JWT validation)

## API Endpoints

- `GET /healthz`
- `GET /api/v1/machine/status`
- `POST /api/v1/projects`
- `GET /api/v1/projects`
- `POST /api/v1/services`
- `GET /api/v1/services?projectId=1`
- `POST /api/v1/commands`
- `POST /api/v1/builds`
- `GET /api/v1/jobs/{jobId}`
- `GET /api/v1/logs?namespace=ns-a&service=api&tail=100`
- `GET /api/v1/status?namespace=ns-a&service=api`

## ตัวอย่าง Header สำหรับยืนยันตัวตน

```bash
-H "X-API-Key: local-admin-key"
```

## ตัวอย่างลำดับการใช้งาน

1. สร้าง project พร้อม namespace และ resource policy
2. สร้าง service ที่เชื่อมกับ project
3. ส่งคำสั่ง command/build เข้าคิวงาน
4. ตรวจสอบสถานะงานจาก endpoint jobs
