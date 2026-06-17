module Main (main) where

import System.Exit (exitWith)
import Yass.CLI (run)

main :: IO ()
main = run >>= exitWith
