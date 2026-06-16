package ai.openwatcher.watchapp.data

interface DebugDemoStore {
    fun current(): DebugDemoScenario
    fun set(value: DebugDemoScenario)
}

class DebugDemoPreferenceStore(
    private val store: KeyValueStore,
) : DebugDemoStore {
    override fun current(): DebugDemoScenario {
        val raw = store.getString(KEY_DEBUG_SCENARIO).orEmpty()
        return DebugDemoScenario.entries.firstOrNull { it.name == raw } ?: DebugDemoScenario.NONE
    }

    override fun set(value: DebugDemoScenario) {
        store.putString(KEY_DEBUG_SCENARIO, value.name)
    }

    companion object {
        private const val KEY_DEBUG_SCENARIO = "debug_demo_scenario"
    }
}

