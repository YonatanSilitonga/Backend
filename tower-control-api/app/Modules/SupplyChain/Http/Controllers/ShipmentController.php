<?php

namespace App\Modules\SupplyChain\Http\Controllers;

use App\Http\Controllers\Controller;
use App\Modules\SupplyChain\Models\Shipment;
use App\Modules\SupplyChain\Models\TrackingEvent;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Str;

class ShipmentController extends Controller
{
    public function index(Request $request): JsonResponse
    {
        $shipments = Shipment::query()
            ->when($request->query('status'), fn ($q, $s) => $q->where('status', $s))
            ->when($request->query('no_resi'), fn ($q, $r) => $q->where('no_resi', 'like', "%$r%"))
            ->when($request->query('asal'), fn ($q, $a) => $q->where('asal', $a))
            ->when($request->query('tujuan'), fn ($q, $t) => $q->where('tujuan', $t))
            ->orderByDesc('created_at')
            ->get();

        return response()->json(['success' => true, 'data' => $shipments]);
    }

    public function store(Request $request): JsonResponse
    {
        $validated = $request->validate([
            'pengirim' => ['required', 'string', 'max:255'],
            'penerima' => ['required', 'string', 'max:255'],
            'asal' => ['required', 'string', 'max:255'],
            'tujuan' => ['required', 'string', 'max:255'],
            'berat_kg' => ['required', 'numeric', 'min:0'],
            'status' => ['sometimes', 'string'],
            'trip_id' => ['nullable', 'string'],
            'vehicle_id' => ['nullable', 'string'],
            'deskripsi' => ['nullable', 'string'],
        ]);

        $validated['no_resi'] = $request->input('no_resi') ?? 'SLB-' . strtoupper(Str::random(10));
        $validated['status'] = $validated['status'] ?? 'pending';

        $shipment = Shipment::create($validated);

        TrackingEvent::create([
            'shipment_id' => $shipment->id,
            'status' => $shipment->status,
            'lokasi' => $shipment->asal,
            'deskripsi' => 'Shipment dibuat',
            'event_time' => now(),
        ]);

        return response()->json(['success' => true, 'data' => $shipment], 201);
    }

    public function show(string $id): JsonResponse
    {
        $shipment = Shipment::find($id);

        if (! $shipment) {
            return response()->json(['success' => false, 'message' => 'Shipment tidak ditemukan'], 404);
        }

        return response()->json(['success' => true, 'data' => $shipment]);
    }

    public function update(Request $request, string $id): JsonResponse
    {
        $shipment = Shipment::find($id);

        if (! $shipment) {
            return response()->json(['success' => false, 'message' => 'Shipment tidak ditemukan'], 404);
        }

        $validated = $request->validate([
            'pengirim' => ['sometimes', 'string', 'max:255'],
            'penerima' => ['sometimes', 'string', 'max:255'],
            'asal' => ['sometimes', 'string', 'max:255'],
            'tujuan' => ['sometimes', 'string', 'max:255'],
            'berat_kg' => ['sometimes', 'numeric', 'min:0'],
            'status' => ['sometimes', 'string'],
            'trip_id' => ['nullable', 'string'],
            'vehicle_id' => ['nullable', 'string'],
            'deskripsi' => ['nullable', 'string'],
        ]);

        $shipment->update($validated);

        return response()->json(['success' => true, 'data' => $shipment]);
    }

    /**
     * Update status + otomatis catat tracking event.
     */
    public function updateStatus(Request $request, string $id): JsonResponse
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

        $shipment->update(['status' => $validated['status']]);

        TrackingEvent::create([
            'shipment_id' => $shipment->id,
            'status' => $validated['status'],
            'lokasi' => $validated['lokasi'] ?? $shipment->tujuan,
            'latitude' => $validated['latitude'] ?? null,
            'longitude' => $validated['longitude'] ?? null,
            'deskripsi' => $validated['deskripsi'] ?? 'Status berubah ke ' . $validated['status'],
            'event_time' => now(),
        ]);

        return response()->json(['success' => true, 'data' => $shipment]);
    }

    public function destroy(string $id): JsonResponse
    {
        $shipment = Shipment::find($id);

        if (! $shipment) {
            return response()->json(['success' => false, 'message' => 'Shipment tidak ditemukan'], 404);
        }

        TrackingEvent::where('shipment_id', $shipment->id)->delete();
        $shipment->delete();

        return response()->json(['success' => true, 'message' => 'Shipment dihapus']);
    }
}
