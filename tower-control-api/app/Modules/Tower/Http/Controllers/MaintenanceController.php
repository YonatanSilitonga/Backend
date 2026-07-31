<?php

namespace App\Modules\Tower\Http\Controllers;

use App\Http\Controllers\Controller;
use App\Modules\Tower\Models\MaintenanceTask;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Str;

class MaintenanceController extends Controller
{
    public function index(Request $request): JsonResponse
    {
        $tasks = MaintenanceTask::query()
            ->when($request->query('status'), fn ($q, $s) => $q->where('status', $s))
            ->when($request->query('tower_id'), fn ($q, $t) => $q->where('tower_id', $t))
            ->get();

        return response()->json(['success' => true, 'data' => $tasks]);
    }

    public function store(Request $request): JsonResponse
    {
        $validated = $request->validate([
            'tower_id' => ['required', 'string'],
            'vendor_id' => ['nullable', 'string'],
            'jenis' => ['required', 'string', 'max:100'],
            'jadwal' => ['required', 'date'],
            'biaya' => ['nullable', 'numeric', 'min:0'],
            'status' => ['sometimes', 'string', 'in:scheduled,in_progress,completed,cancelled'],
            'catatan' => ['nullable', 'string'],
        ]);

        $validated['kode'] = 'MTN-' . strtoupper(Str::random(8));
        $validated['status'] = $validated['status'] ?? 'scheduled';

        $task = MaintenanceTask::create($validated);

        return response()->json(['success' => true, 'data' => $task], 201);
    }

    public function show(string $id): JsonResponse
    {
        $task = MaintenanceTask::with(['tower', 'vendor'])->find($id);

        if (! $task) {
            return response()->json(['success' => false, 'message' => 'Task maintenance tidak ditemukan'], 404);
        }

        return response()->json(['success' => true, 'data' => $task]);
    }

    public function update(Request $request, string $id): JsonResponse
    {
        $task = MaintenanceTask::find($id);

        if (! $task) {
            return response()->json(['success' => false, 'message' => 'Task maintenance tidak ditemukan'], 404);
        }

        $validated = $request->validate([
            'tower_id' => ['sometimes', 'string'],
            'vendor_id' => ['nullable', 'string'],
            'jenis' => ['sometimes', 'string', 'max:100'],
            'jadwal' => ['sometimes', 'date'],
            'biaya' => ['nullable', 'numeric', 'min:0'],
            'status' => ['sometimes', 'string', 'in:scheduled,in_progress,completed,cancelled'],
            'catatan' => ['nullable', 'string'],
        ]);

        $task->update($validated);

        return response()->json(['success' => true, 'data' => $task]);
    }

    public function destroy(string $id): JsonResponse
    {
        $task = MaintenanceTask::find($id);

        if (! $task) {
            return response()->json(['success' => false, 'message' => 'Task maintenance tidak ditemukan'], 404);
        }

        $task->delete();

        return response()->json(['success' => true, 'message' => 'Task maintenance dihapus']);
    }
}
