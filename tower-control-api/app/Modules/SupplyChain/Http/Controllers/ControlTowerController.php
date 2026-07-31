<?php

namespace App\Modules\SupplyChain\Http\Controllers;

use App\Http\Controllers\Controller;
use App\Modules\Armada\Models\Vehicle;
use App\Modules\SupplyChain\Models\Shipment;
use App\Modules\SupplyChain\Models\TrackingEvent;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use MongoDB\BSON\UTCDateTime;

class ControlTowerController extends Controller
{
    /**
     * Ringkasan operasional end-to-end untuk dashboard Control Tower.
     */
    public function summary(Request $request): JsonResponse
    {
        $days = (int) $request->query('days', 7);

        // --- Shipment: breakdown per status (total keseluruhan) ---
        $statusBreakdown = $this->groupCount(Shipment::class, 'status');

        // --- Shipment 7 hari terakhir: tren per hari ---
        $since = now()->subDays($days)->startOfDay();
        $dailyTrend = $this->shipmentTrend($since);

        // --- Top rute (asal -> tujuan) ---
        $topRoutes = Shipment::raw(function ($collection) {
            return $collection->aggregate([
                ['$group' => [
                    '_id' => ['asal' => '$asal', 'tujuan' => '$tujuan'],
                    'total' => ['$sum' => 1],
                ]],
                ['$sort' => ['total' => -1]],
                ['$limit' => 5],
            ])->toArray();
        });

        $routes = array_map(fn ($r) => [
            'asal' => $r['_id']['asal'],
            'tujuan' => $r['_id']['tujuan'],
            'total' => $r['total'],
        ], $topRoutes);

        // --- Status kendaraan ---
        $vehicleBreakdown = $this->groupCount(Vehicle::class, 'status');

        // --- Tracking event terbaru (live feed) ---
        $recentEvents = TrackingEvent::orderByDesc('event_time')->limit(10)->get();

        return response()->json([
            'success' => true,
            'data' => [
                'total_shipments' => Shipment::count(),
                'shipment_by_status' => $statusBreakdown,
                'shipment_trend_days' => $dailyTrend,
                'top_routes' => $routes,
                'vehicle_by_status' => $vehicleBreakdown,
                'recent_events' => $recentEvents,
            ],
        ]);
    }

    /**
     * Count group by field.
     */
    private function groupCount(string $model, string $field): array
    {
        $rows = $model::raw(function ($collection) use ($field) {
            return $collection->aggregate([
                ['$group' => ['_id' => '$' . $field, 'total' => ['$sum' => 1]]],
                ['$sort' => ['total' => -1]],
            ])->toArray();
        });

        return array_map(fn ($r) => [
            'key' => $r['_id'] ?? 'unknown',
            'total' => $r['total'],
        ], $rows);
    }

    /**
     * Tren shipment per hari sejak tanggal tertentu.
     */
    private function shipmentTrend(\Illuminate\Support\Carbon $since): array
    {
        $rows = Shipment::raw(function ($collection) use ($since) {
            return $collection->aggregate([
                ['$match' => ['created_at' => ['$gte' => new UTCDateTime($since->getTimestamp() * 1000)]]],
                ['$group' => [
                    '_id' => [
                        'tahun' => ['$year' => '$created_at'],
                        'bulan' => ['$month' => '$created_at'],
                        'hari' => ['$dayOfMonth' => '$created_at'],
                    ],
                    'total' => ['$sum' => 1],
                ]],
                ['$sort' => ['_id' => 1]],
            ])->toArray();
        });

        $result = [];
        foreach ($rows as $r) {
            $date = sprintf('%04d-%02d-%02d', $r['_id']['tahun'], $r['_id']['bulan'], $r['_id']['hari']);
            $result[] = ['tanggal' => $date, 'total' => $r['total']];
        }

        return $result;
    }
}
