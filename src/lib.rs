pub mod argv;
pub mod error_line;
pub mod errors;
pub mod list;
pub mod query;
pub mod shared;
pub mod validate;
pub mod yaml_parse;

use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, Ordering};

static SIGINT_RECEIVED: AtomicBool = AtomicBool::new(false);
static SIGTERM_RECEIVED: AtomicBool = AtomicBool::new(false);

/// Install signal handlers for SIGPIPE, SIGINT, and SIGTERM.
pub fn install_signal_handlers() {
    // SIGPIPE: ignore it so writes to broken pipes don't crash the process;
    // we handle BrokenPipe errors at the IO level if needed.
    #[cfg(unix)]
    unsafe {
        libc::signal(libc::SIGPIPE, libc::SIG_IGN);
    }

    // SIGINT and SIGTERM: set flags for graceful exit.
    #[cfg(unix)]
    unsafe {
        libc::signal(libc::SIGINT, sigint_handler as *const () as libc::sighandler_t);
        libc::signal(libc::SIGTERM, sigterm_handler as *const () as libc::sighandler_t);
    }
}

#[cfg(unix)]
extern "C" fn sigint_handler(_sig: libc::c_int) {
    SIGINT_RECEIVED.store(true, Ordering::SeqCst);
}

#[cfg(unix)]
extern "C" fn sigterm_handler(_sig: libc::c_int) {
    SIGTERM_RECEIVED.store(true, Ordering::SeqCst);
}

/// Entry point for the yass CLI. Returns a process exit code.
pub fn run() -> i32 {
    install_signal_handlers();

    let result = std::panic::catch_unwind(|| {
        let args: Vec<String> = std::env::args().collect();
        let code = argv::dispatch(&args);

        // Check if a signal was received during execution.
        if SIGINT_RECEIVED.load(Ordering::SeqCst) {
            return errors::EXIT_SIGINT;
        }
        if SIGTERM_RECEIVED.load(Ordering::SeqCst) {
            return errors::EXIT_SIGTERM;
        }

        code
    });

    match result {
        Ok(code) => code,
        Err(panic_info) => {
            // Uncaught panic: emit ErrorLine to stderr and exit 1.
            let message = if let Some(s) = panic_info.downcast_ref::<&str>() {
                s.to_string()
            } else if let Some(s) = panic_info.downcast_ref::<String>() {
                s.clone()
            } else {
                "unexpected panic".to_string()
            };
            let cwd = std::env::current_dir().unwrap_or_else(|_| PathBuf::from("."));
            let e = errors::CliError::new(errors::INTERNAL_UNCAUGHT, format!("internal error: {}", message));
            error_line::emit_error(&e, &cwd);
            errors::EXIT_PROCESSING
        }
    }
}
