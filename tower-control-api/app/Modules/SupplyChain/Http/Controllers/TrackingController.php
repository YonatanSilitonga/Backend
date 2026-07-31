<?php

namespace App\Modules\SupplyChain\Http\Controllers;

use App\Http\Controllers\Controller;
use App\Modules\SupplyChain\Models\Shipment;
use App\Modules\SupplyChain\Models\TrackingEvent;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;

class TrackingController extends Controller
{
    /**
     * Track shipment berdasarkan no resi. Public-friendly (bisa dipakai customer).
     */
    public function track(string $noResi): JsonResponse
    {
        $shipment = Shipment::where('no_resi', $noResi)->first();

        if (! $shipment) {
            return response()->json([
                'success' => false,
                'message' => 'No resi tidak ditemukan',
            ], 404);
        }

        $events = TrackingEvent::where('shipment_id', $shipment->id)
            ->orderByDesc('event_time')
            ->get();

        return response()->json([
            'success' => true,
            'data' => [
                'shipment' => $shipment,
                'riwayat' => $events,
            ],
        ]);
    }

    /**
     * Tambah tracking event manual untuk shipment.
     */
    public function store(Request $request, string $id): JsonResponse
    {
        $shipment = Shipment::find($id);

        if (! $shipment) {
            return response()->json(['success' => false, 'message' => 'Shipment tidak ditemukan'], 404);
        }

        $validated = $request->validate([
            'status' => ['required', 'string'],
            'lokasi' => ['nullable', 'string'],
            'latitude' => ['nullable', 'numeric'],
            'longitude' => ['nullable', 'numeric'],
            'deskripsi' => ['nullable', 'string'],
        ]);

        $event = TrackingEvent::create([
            'shipment_id' => $shipment->id,
            'status' => $validated['status'],
            'lokasi' => $validated['lokasi'] ?? null,
            'latitude' => $validated['latitude'] ?? null,
            'longitude' => $validated['longitude'] ?? null,
            'deskripsi' => $validated['deskripsi'] ?? null,
            'event_time' => now(),
        ]);

        // Sinkronkan status shipment dengan event terbaru
        $shipment->update(['status' => $validated['status']]);

        return response()->json(['success' => true, 'data' => $event], 201);
    }

    /**
     * Riwayat tracking per shipment.
     */
    public function history(string $id): JsonResponse
    {
        $shipment = Shipment::find($id);

        if (! $shipment) {
            return response()->json(['success' => false, 'message' => 'Shipment tidak ditemukan'], 404);
        }

        $events = TrackingEvent::where('shipment_id', $shipment->id)
            ->orderByDesc('event_time')
            ->get();

        return response()->json(['success' => true, 'data' => $events]);
    }
}
