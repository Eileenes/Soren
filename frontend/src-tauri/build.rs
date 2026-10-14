fn main() {
    // generate_context! embeds the macOS dev icon. Recompile when brand assets
    // change so a dev restart cannot silently retain the old Dock icon.
    println!("cargo:rerun-if-changed=icons");
    tauri_build::build()
}
