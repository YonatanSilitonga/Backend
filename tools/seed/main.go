package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"backend/internal/config"
	"backend/internal/database"
)

var db *pgxpool.Pool

func main() {
	ctx := context.Background()
	cfg := config.Load()
	var err error
	db, err = database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// ---------- karyawan ----------
	var idKaryawanAdmin, idKaryawanKapten, idKaryawanDir int64
	insertReturning(ctx, "INSERT INTO karyawan (nik, nama, jabatan, penempatan, tempat_penugasan, shift, status) VALUES ('3171010101900001','Budi Santoso','Admin Operasional','Jakarta','Hub Pusat','Pagi','aktif') RETURNING id_karyawan", &idKaryawanAdmin)
	insertReturning(ctx, "INSERT INTO karyawan (nik, nama, jabatan, penempatan, tempat_penugasan, shift, status) VALUES ('3578010404920004','Slamet Riyadi','Kapten Driver','Surabaya','Gudang Surabaya','Pagi','aktif') RETURNING id_karyawan", &idKaryawanKapten)
	insertReturning(ctx, "INSERT INTO karyawan (nik, nama, jabatan, penempatan, tempat_penugasan, shift, status) VALUES ('3171010000000001','Direktur Operasional','Direktur Operasional','Jakarta','Kantor Pusat','Pagi','aktif') RETURNING id_karyawan", &idKaryawanDir)
	exec(ctx, "INSERT INTO karyawan (nik, nama, jabatan, penempatan, tempat_penugasan, shift, status) VALUES ('3171010202900002','Agus Wijaya','Driver','Jakarta','Hub Pusat','Pagi','aktif')")
	exec(ctx, "INSERT INTO karyawan (nik, nama, jabatan, penempatan, tempat_penugasan, shift, status) VALUES ('3171010303910003','Rudi Hartono','Driver','Jakarta','Hub Pusat','Sore','aktif')")
	exec(ctx, "INSERT INTO karyawan (nik, nama, jabatan, penempatan, tempat_penugasan, shift, status) VALUES ('3578010505930005','Dedi Kurniawan','Driver','Surabaya','Gudang Surabaya','Pagi','aktif')")
	exec(ctx, "INSERT INTO karyawan (nik, nama, jabatan, penempatan, tempat_penugasan, shift, status) VALUES ('1271010606940006','Hendra Gunawan','Driver','Medan','Gudang Medan','Pagi','aktif')")

	// ---------- driver ----------
	var idDrvBudi, idDrvAgus, idDrvSlamet, idDrvRudi int64
	insertReturning(ctx, "INSERT INTO driver (nama_driver, no_hp, no_sim, jenis_sim, status_driver) VALUES ('Budi Santoso','0812-3456-7890','810112345678','B1 Umum','bertugas') RETURNING id_driver", &idDrvBudi)
	insertReturning(ctx, "INSERT INTO driver (nama_driver, no_hp, no_sim, jenis_sim, status_driver) VALUES ('Agus Wijaya','0813-9876-5432','810298765432','B1 Umum','bertugas') RETURNING id_driver", &idDrvAgus)
	insertReturning(ctx, "INSERT INTO driver (nama_driver, no_hp, no_sim, jenis_sim, status_driver) VALUES ('Slamet Riyadi','0857-3333-4444','810498765433','B1 Umum','bertugas') RETURNING id_driver", &idDrvSlamet)
	exec(ctx, "INSERT INTO driver (nama_driver, no_hp, no_sim, jenis_sim, status_driver) VALUES ('Dedi Kurniawan','0811-5555-6666','810512345680','B1 Umum','libur')")
	exec(ctx, "INSERT INTO driver (nama_driver, no_hp, no_sim, jenis_sim, status_driver) VALUES ('Hendra Gunawan','0822-7777-8888','810698765434','B2','libur')")
	insertReturning(ctx, "INSERT INTO driver (nama_driver, no_hp, no_sim, jenis_sim, status_driver) VALUES ('Rudi Hartono','0821-1111-2222','810312345679','B1 Umum','bertugas') RETURNING id_driver", &idDrvRudi)

	// ---------- kendaraan ----------
	var idVeh1, idVeh2, idVeh3, idVeh4 int64
	insertReturning(ctx, "INSERT INTO kendaraan (kode_kendaraan, plat_nomor, jenis_kendaraan, kapasitas_koli, kapasitas_kg, status_kendaraan) VALUES ('TRK-001','B 1234 SLB','Truk Box 6m',80,8000,'berjalan') RETURNING id_kendaraan", &idVeh1)
	insertReturning(ctx, "INSERT INTO kendaraan (kode_kendaraan, plat_nomor, jenis_kendaraan, kapasitas_koli, kapasitas_kg, status_kendaraan) VALUES ('TRK-002','B 5678 SLB','Truk Box 6m',80,8000,'tersedia') RETURNING id_kendaraan", &idVeh2)
	insertReturning(ctx, "INSERT INTO kendaraan (kode_kendaraan, plat_nomor, jenis_kendaraan, kapasitas_koli, kapasitas_kg, status_kendaraan) VALUES ('PKP-001','B 9012 SLB','Pickup Double',30,1500,'maintenance') RETURNING id_kendaraan", &idVeh3)
	insertReturning(ctx, "INSERT INTO kendaraan (kode_kendaraan, plat_nomor, jenis_kendaraan, kapasitas_koli, kapasitas_kg, status_kendaraan) VALUES ('WNG-001','B 3456 SLB','Truk Wingbox 8m',120,12000,'berjalan') RETURNING id_kendaraan", &idVeh4)

	// ---------- seller ----------
	var idSeller1, idSeller2 int64
	insertReturning(ctx, "INSERT INTO seller (kode_seller, nama_seller, alamat, kota, area, pic, no_hp, jam_mulai_pickup, jam_selesai_pickup, forecast_harian, status, latitude, longitude) VALUES ('SLR-001','TITIP AJA','RMM9+49Q, RT.002/RW.003, Poris Plawad, Kec. Batuceper, Kota Tangerang, Banten 15141','Tangerang','TNG Area','Jarot','+62 899-2279-170','08:00:00','17:00:00',600,'aktif', -6.152972, 106.603056) RETURNING id_seller", &idSeller1)
	insertReturning(ctx, "INSERT INTO seller (kode_seller, nama_seller, alamat, kota, area, pic, no_hp, jam_mulai_pickup, jam_selesai_pickup, forecast_harian, status, latitude, longitude) VALUES ('SLR-002','SOMETHING','QJQG+5H7 Cikokol, Kota Tangerang, Banten','Tangerang','TNG Area','Deni','+62 895-3281-77533','08:00:00','17:00:00',450,'aktif', -6.102222, 106.685694) RETURNING id_seller", &idSeller2)
	exec(ctx, "INSERT INTO seller (kode_seller, nama_seller, alamat, kota, area, pic, no_hp, jam_mulai_pickup, jam_selesai_pickup, forecast_harian, status, latitude, longitude) VALUES ('SLR-003','SKI','Jl. Pajajaran XIV No.62, RT.005/RW.005, Gandasari, Kec. Jatiuwung, Kota Tangerang, Banten 15810','Tangerang','TNG Area','Mun','+62 856-0834-9714','08:00:00','17:00:00',500,'aktif', -6.231611, 106.720278)")
	exec(ctx, "INSERT INTO seller (kode_seller, nama_seller, alamat, kota, area, pic, no_hp, jam_mulai_pickup, jam_selesai_pickup, forecast_harian, status, latitude, longitude) VALUES ('SLR-004','CILUPBA','VMXP+477 Benda, Kota Tangerang, Banten','Tangerang','TNG Area','Eko','+62 851-7348-9193','08:00:00','17:00:00',300,'aktif', -6.214750, 106.680556)")
	exec(ctx, "INSERT INTO seller (kode_seller, nama_seller, alamat, kota, area, pic, no_hp, jam_mulai_pickup, jam_selesai_pickup, forecast_harian, status, latitude, longitude) VALUES ('SLR-005','PAYUTRUS KACAMATA','QMPJ+463 Pinang, Kota Tangerang, Banten','Tangerang','TNG Area','gopur','+62 895-3953-20446','08:00:00','17:00:00',400,'aktif', -6.220694, 106.585472)")
	exec(ctx, "INSERT INTO seller (kode_seller, nama_seller, alamat, kota, area, pic, no_hp, jam_mulai_pickup, jam_selesai_pickup, forecast_harian, status, latitude, longitude) VALUES ('SLR-006','BAYUR','RJW3+R63 Periuk Jaya, Kota Tangerang, Banten','Tangerang','TNG Area','siregar','+62 853-8119-6599','08:00:00','17:00:00',350,'aktif', -6.212083, 106.626444)")
	exec(ctx, "INSERT INTO seller (kode_seller, nama_seller, alamat, kota, area, pic, no_hp, jam_mulai_pickup, jam_selesai_pickup, forecast_harian, status, latitude, longitude) VALUES ('SLR-007','LARANGAN CIPADU','QP9C+943 Sudimara Tim., Kota Tangerang, Banten','Tangerang','TNG Area','Junaedi','+62 858-9471-8860','08:00:00','17:00:00',400,'aktif', -6.167750, 106.668778)")

	// ---------- drop_point ----------
	var idDP1, idDP2 int64
	insertReturning(ctx, "INSERT INTO drop_point (kode_dp, nama_drop_point, alamat, koordinator, status) VALUES ('DP-001','Hub Jakarta Pusat','Jl. Gatot Subroto No.1','Koord JKT','aktif') RETURNING id_drop_point", &idDP1)
	insertReturning(ctx, "INSERT INTO drop_point (kode_dp, nama_drop_point, alamat, koordinator, status) VALUES ('DP-002','DC Surabaya Timur','Jl. Rungkut Industri No.2','Koord SBY','aktif') RETURNING id_drop_point", &idDP2)

	// ---------- users (bcrypt) ----------
	hashAdmin, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	exec(ctx, "INSERT INTO users (username, password, role, karyawan_id) VALUES ('admin',$1,'admin',$2)", string(hashAdmin), idKaryawanAdmin)

	hashTowerControl, _ := bcrypt.GenerateFromPassword([]byte("tower123"), bcrypt.DefaultCost)
	exec(ctx, "INSERT INTO users (username, password, role, karyawan_id) VALUES ('tower_control',$1,'tower_control',$2)", string(hashTowerControl), idKaryawanKapten)

	hashDir, _ := bcrypt.GenerateFromPassword([]byte("direktur123"), bcrypt.DefaultCost)
	exec(ctx, "INSERT INTO users (username, password, role, karyawan_id) VALUES ('direktur',$1,'direktur',$2)", string(hashDir), idKaryawanDir)

	hashDriver, _ := bcrypt.GenerateFromPassword([]byte("driver123"), bcrypt.DefaultCost)
	exec(ctx, "INSERT INTO users (username, password, role) VALUES ('driver',$1,'driver')", string(hashDriver))

	hashAwaludin, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	exec(ctx, "INSERT INTO users (username, password, role) VALUES ('AWALUDIN',$1,'driver') ON CONFLICT (username) DO NOTHING", string(hashAwaludin))

	// ---------- ritase ----------
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	var idRitase1, idRitase2, idRitase3, idRitase4 int64
	insertReturning(ctx, "INSERT INTO ritase (kode_ritase, tanggal, id_driver, id_kendaraan, id_seller, id_drop_point, ritase_ke, total_awb, total_koli, jam_berangkat, jam_tiba, status) VALUES ('RTS-0001',$1,$2,$3,$4,$5,1,1200,40,'08:00:00',NULL,'berjalan') RETURNING id_ritase", &idRitase1, today, idDrvBudi, idVeh1, idSeller1, idDP1)
	insertReturning(ctx, "INSERT INTO ritase (kode_ritase, tanggal, id_driver, id_kendaraan, id_seller, id_drop_point, ritase_ke, total_awb, total_koli, jam_berangkat, jam_tiba, status) VALUES ('RTS-0002',$1,$2,$3,$4,$5,2,950,30,'10:00:00','14:30:00','selesai') RETURNING id_ritase", &idRitase2, yesterday, idDrvAgus, idVeh2, idSeller2, idDP2)
	insertReturning(ctx, "INSERT INTO ritase (kode_ritase, tanggal, id_driver, id_kendaraan, id_seller, id_drop_point, ritase_ke, total_awb, total_koli, jam_berangkat, jam_tiba, status) VALUES ('RTS-0003',$1,$2,$3,$4,$5,1,800,25,'09:00:00',NULL,'berjalan') RETURNING id_ritase", &idRitase3, today, idDrvSlamet, idVeh4, idSeller1, idDP2)
	insertReturning(ctx, "INSERT INTO ritase (kode_ritase, tanggal, id_driver, id_kendaraan, id_seller, id_drop_point, ritase_ke, total_awb, total_koli, jam_berangkat, jam_tiba, status) VALUES ('RTS-0004',$1,$2,$3,$4,$5,1,0,0,NULL,NULL,'direncanakan') RETURNING id_ritase", &idRitase4, today, idDrvRudi, idVeh3, idSeller2, idDP1)

	// ---------- ritase_event (timeline status) ----------
	// RTS-0001 (berjalan): berangkat gudang -> sampai gudang -> mulai loading -> selesai loading -> berangkat seller -> sampai seller
	now := time.Now()
	addEvent(ctx, idRitase1, "berangkat_gudang", now.Add(-5*time.Hour))
	addEvent(ctx, idRitase1, "sampai_gudang", now.Add(-4*time.Hour+50*time.Minute))
	addEvent(ctx, idRitase1, "mulai_loading", now.Add(-4*time.Hour))
	addEvent(ctx, idRitase1, "selesai_loading", now.Add(-3*time.Hour+20*time.Minute))
	addEvent(ctx, idRitase1, "berangkat_seller", now.Add(-3*time.Hour))
	addEvent(ctx, idRitase1, "sampai_seller", now.Add(-2*time.Hour+30*time.Minute))

	// RTS-0002 (selesai): timeline lengkap
	addEvent(ctx, idRitase2, "berangkat_gudang", now.Add(-30*time.Hour))
	addEvent(ctx, idRitase2, "sampai_gudang", now.Add(-29*time.Hour))
	addEvent(ctx, idRitase2, "mulai_loading", now.Add(-28*time.Hour+50*time.Minute))
	addEvent(ctx, idRitase2, "selesai_loading", now.Add(-28*time.Hour))
	addEvent(ctx, idRitase2, "berangkat_seller", now.Add(-27*time.Hour+40*time.Minute))
	addEvent(ctx, idRitase2, "sampai_seller", now.Add(-27*time.Hour))
	addEvent(ctx, idRitase2, "mulai_unloading", now.Add(-26*time.Hour+50*time.Minute))
	addEvent(ctx, idRitase2, "selesai_unloading", now.Add(-26*time.Hour+30*time.Minute))
	addEvent(ctx, idRitase2, "selesai_bertugas", now.Add(-25*time.Hour))

	// RTS-0003 (berjalan, lama tanpa update -> trigger alert)
	addEvent(ctx, idRitase3, "berangkat_gudang", now.Add(-10*time.Hour))
	addEvent(ctx, idRitase3, "sampai_gudang", now.Add(-9*time.Hour))
	addEvent(ctx, idRitase3, "mulai_loading", now.Add(-8*time.Hour+30*time.Minute))
	addEvent(ctx, idRitase3, "selesai_loading", now.Add(-8*time.Hour))

	// ---------- armada_tracking ----------
	exec(ctx, "INSERT INTO armada_tracking (id_ritase, id_kendaraan, id_driver, latitude, longitude, kecepatan, arah, status, last_update) VALUES ($1,$2,$3,-6.200000,106.816666,45,90,'berjalan',$4)", idRitase1, idVeh1, idDrvBudi, time.Now())
	exec(ctx, "INSERT INTO armada_tracking (id_ritase, id_kendaraan, id_driver, latitude, longitude, kecepatan, arah, status, last_update) VALUES ($1,$2,$3,-7.250000,112.750000,55,120,'berjalan',$4)", idRitase3, idVeh4, idDrvSlamet, time.Now())

	fmt.Println("SEED DONE ✓")
	fmt.Printf("Login: admin/admin123 (admin), tower_control/tower123 (tower_control), direktur/direktur123 (direktur), driver/driver123 (driver)\n")
}

func addEvent(ctx context.Context, idRitase int64, status string, at time.Time) {
	exec(ctx, "INSERT INTO ritase_event (id_ritase, status, created_at) VALUES ($1,$2,$3)", idRitase, status, at)
}

func exec(ctx context.Context, sql string, args ...interface{}) {
	if _, err := db.Exec(ctx, sql, args...); err != nil {
		log.Fatalf("exec gagal: %v\nSQL: %s", err, sql)
	}
}

func insertReturning(ctx context.Context, sql string, destAndArgs ...interface{}) {
	if len(destAndArgs) < 1 {
		log.Fatalf("insertReturning butuh minimal 1 dest: %s", sql)
	}
	dest := destAndArgs[0]
	args := destAndArgs[1:]
	if err := db.QueryRow(ctx, sql, args...).Scan(dest); err != nil {
		log.Fatalf("insert returning gagal: %v\nSQL: %s", err, sql)
	}
}
