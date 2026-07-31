<?php

namespace Database\Seeders;

use App\Models\User;
use App\Modules\Armada\Models\Driver;
use App\Modules\Armada\Models\Fleet;
use App\Modules\Armada\Models\Trip;
use App\Modules\Armada\Models\Vehicle;
use App\Modules\SupplyChain\Models\Shipment;
use App\Modules\SupplyChain\Models\TrackingEvent;
use App\Modules\Tower\Models\Invoice;
use App\Modules\Tower\Models\MaintenanceTask;
use App\Modules\Tower\Models\Tower;
use App\Modules\Tower\Models\TowerContract;
use App\Modules\Tower\Models\Vendor;
use Illuminate\Database\Seeder;

class DatabaseSeeder extends Seeder
{
    use \Illuminate\Database\Console\Seeds\WithoutModelEvents;

    public function run(): void
    {
        // --- Users ---
        $admin = User::create([
            'name' => 'Admin SLB',
            'email' => 'admin@slb.co.id',
            'password' => 'password123',
            'role' => 'admin',
            'active' => true,
        ]);

        User::create([
            'name' => 'Supervisor Ops',
            'email' => 'supervisor@slb.co.id',
            'password' => 'password123',
            'role' => 'supervisor',
            'active' => true,
        ]);

        // --- Armada ---
        $fleet1 = Fleet::create(['kode' => 'FLT-01', 'nama' => 'Fleet Jakarta', 'lokasi' => 'Jakarta', 'status' => 'active']);
        $fleet2 = Fleet::create(['kode' => 'FLT-02', 'nama' => 'Fleet Surabaya', 'lokasi' => 'Surabaya', 'status' => 'active']);

        $vehicle1 = Vehicle::create(['plat' => 'B 1234 XYZ', 'tipe' => 'Truk Box', 'kapasitas_kg' => 5000, 'fleet_id' => $fleet1->id, 'status' => 'available']);
        $vehicle2 = Vehicle::create(['plat' => 'L 5678 ABC', 'tipe' => 'Pickup', 'kapasitas_kg' => 1000, 'fleet_id' => $fleet2->id, 'status' => 'in_transit']);

        $driver1 = Driver::create(['nama' => 'Budi Santoso', 'nik' => '3171010101900001', 'no_sim' => 'SIM-8812', 'telepon' => '081234567890', 'fleet_id' => $fleet1->id, 'status' => 'on_duty']);
        $driver2 = Driver::create(['nama' => 'Agus Wijaya', 'nik' => '3578010101900002', 'no_sim' => 'SIM-5521', 'telepon' => '085678901234', 'fleet_id' => $fleet2->id, 'status' => 'off']);

        Trip::create(['kode' => 'TRP-001', 'vehicle_id' => $vehicle2->id, 'driver_id' => $driver1->id, 'asal' => 'Surabaya', 'tujuan' => 'Malang', 'jarak_km' => 90, 'status' => 'in_progress', 'started_at' => now()->subHours(2)]);

        // --- Supply Chain ---
        $shipment = Shipment::create([
            'no_resi' => 'SLB-202607310001',
            'pengirim' => 'PT Sentral Logistik Bersama',
            'penerima' => 'CV Maju Jaya',
            'asal' => 'Jakarta',
            'tujuan' => 'Bandung',
            'berat_kg' => 250,
            'status' => 'in_transit',
        ]);

        TrackingEvent::create([
            'shipment_id' => $shipment->id,
            'status' => 'picked_up',
            'lokasi' => 'Jakarta',
            'deskripsi' => 'Paket diambil dari gudang',
            'event_time' => now()->subDay(),
        ]);

        TrackingEvent::create([
            'shipment_id' => $shipment->id,
            'status' => 'in_transit',
            'lokasi' => 'Cikampek',
            'latitude' => -6.4058,
            'longitude' => 107.1582,
            'deskripsi' => 'Dalam perjalanan ke Bandung',
            'event_time' => now()->subHours(3),
        ]);

        // --- Tower Fisik ---
        $vendor = Vendor::create(['nama' => 'PT Towerindo Pratama', 'kontak' => 'Rudi', 'telepon' => '021-5551234', 'spesialisasi' => 'Maintenance & Sewa Tower', 'status' => 'active']);

        $tower = Tower::create(['kode' => 'TWR-001', 'nama' => 'Tower Cikarang', 'lokasi' => 'Cikarang, Bekasi', 'tipe' => 'Lattice', 'tinggi_m' => 72, 'jumlah_tenant' => 3, 'status' => 'active']);

        $contract = TowerContract::create([
            'kode' => 'KTR-A1B2C3D4',
            'tower_id' => $tower->id,
            'vendor_id' => $vendor->id,
            'tipe_sewa' => 'Roof Top',
            'biaya_bulanan' => 25000000,
            'tanggal_mulai' => now()->subMonths(6),
            'tanggal_selesai' => now()->addMonths(18),
            'status' => 'active',
        ]);

        MaintenanceTask::create([
            'kode' => 'MTN-0001',
            'tower_id' => $tower->id,
            'vendor_id' => $vendor->id,
            'jenis' => 'Inspeksi Rutin',
            'jadwal' => now()->addWeek(),
            'biaya' => 1500000,
            'status' => 'scheduled',
        ]);

        Invoice::create([
            'no_invoice' => 'INV-000001',
            'contract_id' => $contract->id,
            'vendor_id' => $vendor->id,
            'tower_id' => $tower->id,
            'periode' => now()->format('Y-m'),
            'jumlah' => 25000000,
            'status' => 'unpaid',
            'due_date' => now()->endOfMonth(),
        ]);

        $this->command->info('Seeder selesai. Login admin: admin@slb.co.id / password123');
    }
}
